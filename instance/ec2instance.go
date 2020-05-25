package instance

import (
	"log"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/jbrt/ec2cryptomatic/constants"
)

// ec2Instance is the main type of that package. Will be returned by new.
// It contains all data relevent for an ec2instance
type ec2Instance struct {
	instanceID *string
	client     *ec2.EC2
	describe   *ec2.Instance
}

// GetEBSVolumes returns EBS volumes mapped with this instance
func (e ec2Instance) GetEBSVolumes() []*ec2.InstanceBlockDeviceMapping {
	return e.describe.BlockDeviceMappings
}

// IsStopped will check if the instance is correcly stopped
func (e ec2Instance) IsStopped() bool {
	stopped := true
	if *e.describe.State.Name != "stopped" {
		return !stopped
	}
	return stopped
}

// IsSupportsEncryptedVolumes will check if the instance supports EBS encrypted volumes (not all instances types support that).
func (e ec2Instance) IsSupportsEncryptedVolumes() bool {
	compatible := true
	unsupportedInstanceTypes := []string{"c1.", "m1.", "m2.", "t1."}
	for _, instance := range unsupportedInstanceTypes {
		if strings.HasPrefix(*e.describe.InstanceType, instance) {
			return !compatible
		}
	}
	return compatible

}

// StartInstance will... start the instance. What a surprise ! :-)
func (e ec2Instance) StartInstance() error {
	log.Println("-- Start instance " + *e.instanceID)
	input := &ec2.StartInstancesInput{InstanceIds: []*string{aws.String(*e.instanceID)}}
	_, err := e.client.StartInstances(input)
	if err != nil {
		return err
	}
	return nil
}

func (e ec2Instance) SwapBlockDevice(source *ec2.InstanceBlockDeviceMapping, target *ec2.Volume) error {
	detach := &ec2.DetachVolumeInput{VolumeId: aws.String(*source.Ebs.VolumeId)}
	_, errDetach := e.client.DetachVolume(detach)
	if errDetach != nil {
		return errDetach
	}

	waiterMaxAttempts := request.WithWaiterMaxAttempts(constants.InstanceMaxAttempts)
	errWaiter := e.client.WaitUntilVolumeAvailableWithContext(
		aws.BackgroundContext(),
		&ec2.DescribeVolumesInput{VolumeIds: []*string{source.Ebs.VolumeId}},
		waiterMaxAttempts)

	if errWaiter != nil {
		return errWaiter
	}

	attach := &ec2.AttachVolumeInput{
		Device:     aws.String(*source.DeviceName),
		InstanceId: aws.String(*e.instanceID),
		VolumeId:   aws.String(*target.VolumeId),
	}

	volumeAttach, errAttach := e.client.AttachVolume(attach)
	if errAttach != nil {
		return errAttach
	}

	// FIXME: apparently this part doesn't works. DeleteOnTermination bit isn't set after that code
	if *source.Ebs.DeleteOnTermination {
		volumeAttach.SetDeleteOnTermination(true)
	}

	return nil
}

// New returns a well contruct EC2Instance object instance
func New(session *session.Session, instanceID string) (*ec2Instance, error) {

	// Trying to describe the given instance as security mechanism (instance is exists ? credentials are ok ?)
	client := ec2.New(session)
	input := &ec2.DescribeInstancesInput{InstanceIds: []*string{aws.String(instanceID)}}
	describe, err := client.DescribeInstances(input)
	if err != nil {
		log.Fatal("-- Cannot find EC2 instance " + instanceID)
		return &ec2Instance{}, err
	}

	instance := &ec2Instance{
		instanceID: aws.String(instanceID),
		client:     client,
		describe:   describe.Reservations[0].Instances[0],
	}

	return instance, nil
}
