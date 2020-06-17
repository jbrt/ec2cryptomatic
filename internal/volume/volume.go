package volume

import (
	"log"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/jbrt/ec2cryptomatic/constants"
)

// eBSVolumeToEncrypt contains all needed information for encrypting an EBS volume
type eBSVolumeToEncrypt struct {
	volumeID *string
	kmsKeyID string
	client   *ec2.EC2
	describe *ec2.Volume
}

// getTagSpecifications will returns tags from volumes by filtering out AWS specific tags (aws:xxx)
func (v eBSVolumeToEncrypt) getTagSpecifications() []*ec2.TagSpecification {
	resourceType := "volume"
	var tags []*ec2.Tag
	//tags := []*ec2.Tag{}

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

// takeSnapshot will take a snapshot from the given volume & wait until this snapshot is completed
func (v eBSVolumeToEncrypt) takeSnapshot() (*ec2.Snapshot, error) {
	snapShotInput := &ec2.CreateSnapshotInput{
		Description: aws.String("EC2Cryptomatict temporary snapshot for " + *v.volumeID),
		VolumeId:    v.describe.VolumeId,
	}

	snapshot, err := v.client.CreateSnapshot(snapShotInput)
	if err != nil {
		return nil, err
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

// DeleteVolume will delete the given volume
func (v eBSVolumeToEncrypt) DeleteVolume() error {
	log.Println("--- Delete volume " + *v.volumeID)
	_, err := v.client.DeleteVolume(&ec2.DeleteVolumeInput{VolumeId: v.volumeID})
	if err != nil {
		return err
	}
	return nil
}

// EncryptVolume will produce an encrypted version of the volume
func (v eBSVolumeToEncrypt) EncryptVolume() (*ec2.Volume, error) {
	log.Println("--- Start encryption process for volume " + *v.volumeID)
	encrypted := true
	snapshot, err := v.takeSnapshot()
	if err != nil {
		return nil, err
	}
	volumeInput := &ec2.CreateVolumeInput{
		AvailabilityZone: aws.String(*v.describe.AvailabilityZone),
		SnapshotId:       aws.String(*snapshot.SnapshotId),
		VolumeType:       aws.String(*v.describe.VolumeType),
		Encrypted:        &encrypted,
		KmsKeyId:         &v.kmsKeyID,
	}

	// Adding tags if needed
	tagsWithoutAwsDedicatedTags := v.getTagSpecifications()
	if tagsWithoutAwsDedicatedTags != nil {
		volumeInput.TagSpecifications = tagsWithoutAwsDedicatedTags
	}

	// If volume is IO, let's get the IOPS parameter
	if strings.HasPrefix(*v.describe.VolumeType, "io") {
		log.Println("--- This volumes is IO one let's set IOPS to ", *v.describe.Iops)
		volumeInput.Iops = aws.Int64(*v.describe.Iops)
	}

	volume, err := v.client.CreateVolume(volumeInput)
	if err != nil {
		return nil, err
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

// IsEncrypted will returns true if the given volume is already encrypted
func (v eBSVolumeToEncrypt) IsEncrypted() bool {
	return *v.describe.Encrypted
}

// New returns a well contruct EC2Instance object instance
func New(session *session.Session, volumeID string, kmsKeyID string) (*eBSVolumeToEncrypt, error) {

	// Trying to describe the given instance as security mechanism (instance is exists ? credentials are ok ?)
	client := ec2.New(session)
	input := &ec2.DescribeVolumesInput{VolumeIds: []*string{aws.String(volumeID)}}
	describe, err := client.DescribeVolumes(input)
	if err != nil {
		log.Fatal("--- Cannot get information from volume " + volumeID)
		return &eBSVolumeToEncrypt{}, err
	}

	volume := &eBSVolumeToEncrypt{
		volumeID: aws.String(volumeID),
		client:   client,
		describe: describe.Volumes[0],
		kmsKeyID: kmsKeyID,
	}

	return volume, nil
}
