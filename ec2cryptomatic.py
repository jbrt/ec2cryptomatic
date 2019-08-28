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

    def __init__(self, region: str, instance: str, key: str):
        """
        Constructor
        :param region: the AWS region where the instance is
        :param instance: one instance-id
        :param key: the AWS KMS Key to be used to encrypt the volume
        """
        self._logger = logging.getLogger('ec2-cryptomatic')
        self._logger.setLevel(logging.DEBUG)
        self._kms_key = key

        self._ec2_client = boto3.client('ec2', region_name=region)
        self._ec2_resource = boto3.resource('ec2', region_name=region)
        self._region = region
        self._instance = self._ec2_resource.Instance(id=instance)

        # Volumes
        self._snapshot = None
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

    def _start_instance(self):
        try:
            self._logger.info('-- Starting instance %s' % self._instance.id)
            self._ec2_client.start_instances(InstanceIds=[self._instance.id])
            self._logger.info('-- Instance %s started' % self._instance.id)
        except ClientError:
            raise

    def _cleanup(self, device, discard_source):
        """
        Delete the temporary objects
        :param device: the original device to delete
        """

        self._logger.info('-- Cleanup of resources')
        self._wait_volume.wait(VolumeIds=[device.id])

        if discard_source:
            self._logger.info('--- Deleting unencrypted volume %s' % device.id)
            device.delete()

        else:
            self._logger.info('--- Preserving unencrypted volume %s' % device.id)

        self._snapshot.delete()

    def _create_volume(self, snapshot, original_device):
        """
        Create an encrypted volume from an encrypted snapshot
        :param snapshot: an encrypted snapshot
        :param original_device: device where take additional information
        """
        vol_args = {'SnapshotId': snapshot.id,
                    'VolumeType': original_device.volume_type,
                    'AvailabilityZone': original_device.availability_zone,
                    'Encrypted': True,
                    'KmsKeyId': self._kms_key}

        if original_device.volume_type.startswith('io'):
            vol_args['Iops'] = original_device.iops

        self._logger.info('-- Creating an encrypted volume from %s' % snapshot.id)
        volume = self._ec2_resource.create_volume(**vol_args)
        self._wait_volume.wait(VolumeIds=[volume.id])

        if original_device.tags:
            volume.create_tags(Tags=original_device.tags)

        return volume

    def _swap_device(self, old_volume, new_volume):
        """
        Swap the old device with the new encrypted one
        :param old_volume: volume to detach from the instance
        :param new_volume: volume to attach to the instance
        """

        self._logger.info('-- Swap the old volume and the new one')
        device = old_volume.attachments[0]['Device']
        self._instance.detach_volume(Device=device, VolumeId=old_volume.id)
        self._wait_volume.wait(VolumeIds=[old_volume.id])
        self._instance.attach_volume(Device=device, VolumeId=new_volume.id)

    def _take_snapshot(self, device):
        """
        Take the first snapshot from the volume to encrypt
        :param device: EBS device to encrypt
        """

        self._logger.info('-- Take a snapshot for volume %s' % device.id)
        snapshot = device.create_snapshot(Description='snap of %s' % device.id)
        self._wait_snapshot.wait(SnapshotIds=[snapshot.id])
        return snapshot

    def start_encryption(self, discard_source):
        """ Launch encryption process """

        self._logger.info('\nStart to encrypt instance %s\n' % self._instance.id)

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

            self._logger.info('- Let\'s encrypt volume %s ' % device.id)

            # Keep in mind if DeleteOnTermination is need
            delete_flag = device.attachments[0]['DeleteOnTermination']
            flag_on = {'DeviceName': device.attachments[0]['Device'],
                       'Ebs': {'DeleteOnTermination':  delete_flag}}

            # First we have to take a snapshot from the original device
            self._snapshot = self._take_snapshot(device)
            # Create a new encrypted volume from that snapshot
            self._volume = self._create_volume(self._snapshot, device)
            # Finally, swap the old-device for the new one
            self._swap_device(device, self._volume)
            # It's time to tidy up !
            self._cleanup(device, discard_source)
            # starting the stopped instance
             
            if not discard_source:
                self._logger.info('- Tagging legacy volume %s with replacement '
                                  'id %s' % (device.id, self._volume.id))
                device.create_tags(
                    Tags=[
                        {
                            'Key': 'encryptedReplacement',
                            'Value': self._volume.id
                        },
                    ]
                )

            if delete_flag:
                self._logger.info('-- Put flag DeleteOnTermination on volume')
                self._instance.modify_attribute(BlockDeviceMappings=[flag_on])
        self._start_instance()
        self._logger.info('End of work on instance %s\n' % self._instance.id)


def main(args: argparse.Namespace):
    """
    Main program
    :param args: arguments from CLI
    :return: None
    """

    for instance in args.instances:
        try:
            EC2Cryptomatic(args.region,
                           instance,
                           args.key).start_encryption(args.discard_source)

        except (EndpointConnectionError, ValueError) as error:
            logger.error('Problem with your AWS region ? (%s)' % error)
            sys.exit(1)

        except (ClientError, TypeError) as error:
            logger.error('Problem with the instance (%s)' % error)
            continue


def parse_arguments() -> argparse.Namespace:
    """
    Parse arguments from CLI
    """
    description = 'EC2Cryptomatic - Encrypt EBS volumes from EC2 instances'
    parser = argparse.ArgumentParser(description=description)
    parser.add_argument('-r', '--region', help='AWS Region', required=True)
    parser.add_argument('-i', '--instances', nargs='+',
                        help='Instance to encrypt', required=True)
    parser.add_argument('-k', '--key', help="KMS Key ID. For alias, add prefix 'alias/'", default='alias/aws/ebs')
    parser.add_argument('-ds', '--discard_source', action='store_true', default=False,
                        help='Discard source volume after encryption (default: False)')
    return parser.parse_args()


if __name__ == '__main__':
    main(parse_arguments())
