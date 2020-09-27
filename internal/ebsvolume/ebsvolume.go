package ebsvolume

import (
	"log"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/jbrt/ec2cryptomatic/constants"
)

// VolumeToEncrypt contains all needed information for encrypting an EBS volume
type VolumeToEncrypt struct {
	volumeID *string
	client   *ec2.EC2
	describe *ec2.Volume
}

// getTagSpecifications will returns tags from volumes by filtering out AWS specific tags (aws:xxx)
func (v VolumeToEncrypt) getTagSpecifications() []*ec2.TagSpecification {
	resourceType := "volume"
	var tags []*ec2.Tag

	if v.describe.Tags == nil {
		return nil
	}

	for _, val := range v.describe.Tags {
		if !strings.HasPrefix(*val.Key, "aws:") {
			tags = append(tags, val)
		}
	}

	return []*ec2.TagSpecification{{ResourceType: &resourceType, Tags: tags}}

}

// takeSnapshot will take a snapshot from the given EBS volume & wait until this snapshot is completed
func (v VolumeToEncrypt) takeSnapshot() (*ec2.Snapshot, error) {
	snapShotInput := &ec2.CreateSnapshotInput{
		Description: aws.String("EC2Cryptomatic temporary snapshot for " + *v.volumeID),
		VolumeId:    v.describe.VolumeId,
	}

	snapshot, errSnapshot := v.client.CreateSnapshot(snapShotInput); if errSnapshot != nil {
		return nil, errSnapshot
	}

	waiterMaxAttempts := request.WithWaiterMaxAttempts(constants.VolumeMaxAttempts)
	errWaiter := v.client.WaitUntilSnapshotCompletedWithContext(
		aws.BackgroundContext(),
		&ec2.DescribeSnapshotsInput{SnapshotIds: []*string{snapshot.SnapshotId}},
		waiterMaxAttempts)

	if errWaiter != nil {
		return nil, errWaiter
	}
	return snapshot, nil
}

// DeleteVolume will delete the given EBS volume
func (v VolumeToEncrypt) DeleteVolume() error {
	log.Println("---> Delete volume " + *v.volumeID)
	if _, errDelete := v.client.DeleteVolume(&ec2.DeleteVolumeInput{VolumeId: v.volumeID}); errDelete != nil {
		return errDelete
	}
	return nil
}

// EncryptVolume will produce an encrypted version of the EBS volume
func (v VolumeToEncrypt) EncryptVolume(kmsKeyID string) (*ec2.Volume, error) {
	log.Println("---> Start encryption process for volume " + *v.volumeID)
	encrypted := true
	snapshot, errSnapshot := v.takeSnapshot(); if errSnapshot != nil {
		return nil, errSnapshot
	}

	volumeInput := &ec2.CreateVolumeInput{
		AvailabilityZone: aws.String(*v.describe.AvailabilityZone),
		SnapshotId:       aws.String(*snapshot.SnapshotId),
		VolumeType:       aws.String(*v.describe.VolumeType),
		Encrypted:        &encrypted,
		KmsKeyId:         aws.String(kmsKeyID),
	}

	// Adding tags if needed
	if tagsWithoutAwsDedicatedTags := v.getTagSpecifications(); tagsWithoutAwsDedicatedTags != nil {
		volumeInput.TagSpecifications = tagsWithoutAwsDedicatedTags
	}

	// If EBS volume is IO, let's get the IOPs parameter
	if strings.HasPrefix(*v.describe.VolumeType, "io") {
		log.Println("---> This volumes is IO one let's set IOPs to ", *v.describe.Iops)
		volumeInput.Iops = aws.Int64(*v.describe.Iops)
	}

	volume, errVolume := v.client.CreateVolume(volumeInput); if errVolume != nil {
		return nil, errVolume
	}

	waiterMaxAttempts := request.WithWaiterMaxAttempts(constants.VolumeMaxAttempts)
	errWaiter := v.client.WaitUntilVolumeAvailableWithContext(
		aws.BackgroundContext(),
		&ec2.DescribeVolumesInput{VolumeIds: []*string{volume.VolumeId}},
		waiterMaxAttempts)

	if errWaiter != nil {
		return nil, errWaiter
	}

	// Before ends, delete the temporary snapshot
	_, _ = v.client.DeleteSnapshot(&ec2.DeleteSnapshotInput{SnapshotId: snapshot.SnapshotId})

	return volume, nil
}

// IsEncrypted will returns true if the given EBS volume is already encrypted
func (v VolumeToEncrypt) IsEncrypted() bool {
	return *v.describe.Encrypted
}

// New returns a well construct EC2Instance object ec2instance
func New(ec2Client *ec2.EC2, volumeID string) (*VolumeToEncrypt, error) {
	// Trying to describe the given ec2instance as security mechanism (ec2instance is exists ? credentials are ok ?)
	volumeInput := &ec2.DescribeVolumesInput{VolumeIds: []*string{aws.String(volumeID)}}
	describe, errDescribe := ec2Client.DescribeVolumes(volumeInput); if errDescribe != nil {
		log.Println("---> Cannot get information from volume " + volumeID)
		return nil, errDescribe
	}

	volume := &VolumeToEncrypt{
		volumeID: aws.String(volumeID),
		client:   ec2Client,
		describe: describe.Volumes[0],
	}

	return volume, nil
}
