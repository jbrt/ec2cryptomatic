#!/usr/bin/env python3
# coding: utf-8

import argparse
import boto3
import logging
import sys
from botocore.exceptions import ClientError
from botocore.exceptions import EndpointConnectionError

# Define the global logger

logger = logging.getLogger('ec2-cryptomatic')
logger.setLevel(logging.DEBUG)
stream_handler = logging.StreamHandler()
stream_handler.setLevel(logging.DEBUG)
logger.addHandler(stream_handler)


class EC2Cryptomatic(object):
    """ Encrypt EBS volumes from an EC2 instance """

    def __init__(self, region: str, instance: str):
        """ Constructor
            :param region: the AWS region where the instance is
            :param instance: one instance-id
        """
        self._logger = logging.getLogger('ec2-cryptomatic')
        self._logger.setLevel(logging.DEBUG)

        self._ec2_client = boto3.client('ec2', region_name=region)
        self._ec2_resource = boto3.resource('ec2', region_name=region)
        self._region = region
        self._instance = self._ec2_resource.Instance(id=instance)

        # Volumes
        self._snapshot = None
        self._encrypted = None
        self._volume = None

        # Waiters
        self._wait_snapshot = self._ec2_client.get_waiter('snapshot_completed')
        self._wait_volume = self._ec2_client.get_waiter('volume_available')

        # Do some pre-check : instances must exists and be stopped
        self._instance_is_exists()
        self._instance_is_stopped()

    def _instance_is_exists(self):
        try:
            self._ec2_client.describe_instances(InstanceIds=[self._instance.id])
        except ClientError:
            raise

    def _instance_is_stopped(self):
        if self._instance.state['Name'] != 'stopped':
            raise TypeError('Instance still running ! please stop it.')

    def _cleanup(self, device, discard_source):
        """ Delete the temporary objects
            :param device: the original device to delete
        """

        self._logger.info('->Cleanup of resources')
        self._wait_volume.wait(VolumeIds=[device.id])

        if discard_source:
            self._logger.info('-->Deleting unencrypted volume %s' % device.id)
            device.delete()

        else:
            self._logger.info('-->Preserving unencrypted volume %s' % device.id)

        self._snapshot.delete()
        self._encrypted.delete()

    def _create_volume(self, snapshot, original_device):
        """ Create an encrypted volume from an encrypted snapshot
            :param snapshot: an encrypted snapshot
            :param original_device: device where take additionnal informations
        """

        self._logger.info('->Creating an encrypted volume from %s' % snapshot.id)
        volume = self._ec2_resource.create_volume(
                            SnapshotId=snapshot.id,
                            VolumeType=original_device.volume_type,
                            AvailabilityZone=original_device.availability_zone)
        self._wait_volume.wait(VolumeIds=[volume.id])

        if original_device.tags:
            volume.create_tags(Tags=original_device.tags)

        return volume

    def _encrypt_snapshot(self, snapshot):
        """ Copy and encrypt a snapshot
            :param snapshot: snapshot to copy
        """

        self._logger.info('->Copy the snapshot %s and encrypt it' % snapshot.id)
        snap_id = snapshot.copy(Description='encrypted copy of %s' % snapshot.id,
                                Encrypted=True, SourceRegion=self._region)
        snapshot = self._ec2_resource.Snapshot(snap_id['SnapshotId'])
        self._wait_snapshot.wait(SnapshotIds=[snapshot.id])
        return snapshot

    def _swap_device(self, old_volume, new_volume):
        """ Swap the old device with the new encrypted one
            :param old_volume: volume to detach from the instance
            :param new_volume: volume to attach to the instance
        """

        self._logger.info('->Swap the old volume and the new one')
        device = old_volume.attachments[0]['Device']
        self._instance.detach_volume(Device=device, VolumeId=old_volume.id)
        self._wait_volume.wait(VolumeIds=[old_volume.id])
        self._instance.attach_volume(Device=device, VolumeId=new_volume.id)

    def _take_snapshot(self, device):
        """ Take the first snapshot from the volume to encrypt
            :param device: EBS device to encrypt
        """

        self._logger.info('->Take a first snapshot for volume %s' % device.id)
        snapshot = device.create_snapshot(Description='snap of %s' % device.id)
        self._wait_snapshot.wait(SnapshotIds=[snapshot.id])
        return snapshot

    def start_encryption(self, discard_source):
        """ Launch encryption process """

        self._logger.info('Start to encrypt instance %s' % self._instance.id)

        # We encrypt only EC2 EBS-backed. Support of instance store will be
        # added later
        for device in self._instance.block_device_mappings:
            if 'Ebs' not in device:
                msg = '%s: Skip %s not an EBS device' % (self._instance.id,
                                                         device['VolumeId'])
                self._logger.warning(msg)
                continue

        for device in self._instance.volumes.all():
            if device.encrypted:
                msg = '%s: Volume %s already encrypted' % (self._instance.id,
                                                           device.id)
                self._logger.warning(msg)
                continue

            self._logger.info('>Let\'s encrypt volume %s ' % device.id)

            # Keep in mind if DeleteOnTermination is need
            delete_flag = device.attachments[0]['DeleteOnTermination']
            flag_on = {'DeviceName': device.attachments[0]['Device'],
                       'Ebs': {'DeleteOnTermination':  delete_flag}}

            # First we have to take a snapshot from the original device
            self._snapshot = self._take_snapshot(device)
            # Then, copy this snapshot and encrypt it
            self._encrypted = self._encrypt_snapshot(self._snapshot)
            # Create a new volume from that encrypted snapshot
            self._volume = self._create_volume(self._encrypted, device)
            # Finally, swap the old-device for the new one
            self._swap_device(device, self._volume)
            # It's time to tidy up !
            self._cleanup(device, discard_source)
            
            if not discard_source:
                self._logger.info('>Tagging legacy volume %s with replacement '
                                  'id %s' % (device.id, self._volume.id))
                device.create_tags(
                    Tags=[
                        {
                            'Key': 'ec2cryptomaticReplacement',
                            'Value': self._volume.id
                        },
                    ]
                )

            if delete_flag:
                self._logger.info('->Put flag DeleteOnTermination on volume')
                self._instance.modify_attribute(BlockDeviceMappings=[flag_on])

        self._logger.info('End of work on instance %s\n' % self._instance.id)


def main(arguments):
    """ Start the main program """

    for instance in arguments.instances:
        try:
            EC2Cryptomatic(arguments.region, instance).start_encryption(arguments.discard_source)

        except (EndpointConnectionError, ValueError) as error:
            logger.error('Problem with your AWS region ? (%s)' % error)
            sys.exit(1)

        except (ClientError, TypeError) as error:
            logger.error('Problem with the instance (%s)' % error)
            continue


if __name__ == '__main__':
    description = 'EC2Cryptomatic - Encrypt EBS volumes from EC2 instances'
    parser = argparse.ArgumentParser(description=description)
    parser.add_argument('-r', '--region', help='AWS Region', required=True)
    parser.add_argument('-i', '--instances', nargs='+',
                        help='Instance to encrypt', required=True)
    parser.add_argument('-ds', '--discard_source', action='store_true', default=False,
                        help='Discard source volume after encryption (default: False)')
    args = parser.parse_args()
    main(args)
