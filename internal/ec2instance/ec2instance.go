package ec2instance

import (
	"log"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/jbrt/ec2cryptomatic/constants"
	"github.com/jbrt/ec2cryptomatic/internal/ebsvolume"
)

var unsupportedInstanceTypes = []string{"c1.", "m1.", "m2.", "t1."}

// Ec2Instance is the main type of that package. Will be returned by new.
// It contains all data relevant for an ec2instance
type Ec2Instance struct {
	InstanceID       *string
	ec2client        *ec2.EC2
	describeInstance *ec2.Instance
}

// GetEBSMappedVolumes returns EBS volumes mapped with this ec2instance
func (e Ec2Instance) GetEBSMappedVolumes() []*ec2.InstanceBlockDeviceMapping {
	return e.describeInstance.BlockDeviceMappings
}

// GetEBSVolume returns a specific EBS volume with high level methods
func (e Ec2Instance) GetEBSVolume(volumeID string) (*ebsvolume.VolumeToEncrypt, error) {
	ebsVolume, volumeError := ebsvolume.New(e.ec2client, volumeID);	if volumeError != nil {
		return nil, volumeError
	}
	return ebsVolume, nil
}

// IsStopped will check if the ec2instance is correctly stopped
func (e Ec2Instance) IsStopped() bool {
	if *e.describeInstance.State.Name != "stopped" {
		return false
	}
	return true
}

// IsSupportsEncryptedVolumes will check if the ec2instance supports EBS encrypted volumes (not all instances types support that).
func (e Ec2Instance) IsSupportsEncryptedVolumes() bool {
	for _, instance := range unsupportedInstanceTypes {
		if strings.HasPrefix(*e.describeInstance.InstanceType, instance) {
			return false
		}
	}
	return true
}

// StartInstance will... start the ec2instance. What a surprise ! :-)
func (e Ec2Instance) StartInstance() error {
	log.Println("-- Start ec2instance " + *e.InstanceID)
	input := &ec2.StartInstancesInput{InstanceIds: []*string{aws.String(*e.InstanceID)}}
	if _, errStart := e.ec2client.StartInstances(input); errStart != nil {
		return errStart
	}
	return nil
}

//SwapBlockDevice will swap two EBS volumes from an EC2 ec2instance
func (e Ec2Instance) SwapBlockDevice(source *ec2.InstanceBlockDeviceMapping, target *ec2.Volume) error {
	detach := &ec2.DetachVolumeInput{VolumeId: aws.String(*source.Ebs.VolumeId)}
	if _, errDetach := e.ec2client.DetachVolume(detach); errDetach != nil {
		return errDetach
	}

	waiterMaxAttempts := request.WithWaiterMaxAttempts(constants.InstanceMaxAttempts)
	errWaiter := e.ec2client.WaitUntilVolumeAvailableWithContext(
		aws.BackgroundContext(),
		&ec2.DescribeVolumesInput{VolumeIds: []*string{source.Ebs.VolumeId}},
		waiterMaxAttempts)

	if errWaiter != nil {
		return errWaiter
	}

	attach := &ec2.AttachVolumeInput{
		Device:     aws.String(*source.DeviceName),
		InstanceId: aws.String(*e.InstanceID),
		VolumeId:   aws.String(*target.VolumeId),
	}

	if _, errAttach := e.ec2client.AttachVolume(attach); errAttach != nil {
		return errAttach
	}

	if *source.Ebs.DeleteOnTermination {

		mappingSpecification := ec2.InstanceBlockDeviceMappingSpecification{
			DeviceName: aws.String(*source.DeviceName),
			Ebs: &ec2.EbsInstanceBlockDeviceSpecification{
				DeleteOnTermination: aws.Bool(true),
				VolumeId:            target.VolumeId,
			},
		}

		attributeInput := ec2.ModifyInstanceAttributeInput{
			BlockDeviceMappings: []*ec2.InstanceBlockDeviceMappingSpecification{&mappingSpecification},
			InstanceId:          e.InstanceID,
		}

		requestModify, _ := e.ec2client.ModifyInstanceAttributeRequest(&attributeInput)

		if errorRequest := requestModify.Send(); errorRequest != nil {
			return errorRequest
		}

	}

	return nil
}

// New returns a well construct EC2Instance object ec2instance
func New(session *session.Session, instanceID string) (*Ec2Instance, error) {
	log.Println("Let's encrypt EC2 instance " + instanceID)

	// Trying to describeInstance the given ec2instance as security mechanism (ec2instance is exists ? credentials are ok ?)
	ec2client := ec2.New(session)
	ec2Input := &ec2.DescribeInstancesInput{InstanceIds: []*string{aws.String(instanceID)}}

	describe, errDescribe := ec2client.DescribeInstances(ec2Input); if errDescribe != nil {
		log.Println("-- Cannot find EC2 instance " + instanceID)
		return nil, errDescribe
	}

	instance := &Ec2Instance{
		InstanceID:       aws.String(instanceID),
		ec2client:        ec2client,
		describeInstance: describe.Reservations[0].Instances[0],
	}

	return instance, nil
}
