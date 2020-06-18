package algorithm

import (
	"errors"
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/jbrt/ec2cryptomatic/internal/instance"
	"github.com/jbrt/ec2cryptomatic/internal/volume"
)

// EncryptInstance will takes an instanceID and encrypt all the related EBS volumes
func EncryptInstance(instanceID, region, kmsKeyAlias string, sourceDiscard bool) error {
	fmt.Print("\t\t-=[ EC2Cryptomatic ]=-\n")

	awsSession, err := session.NewSession(&aws.Config{Region: aws.String(region)})
	if err != nil {
		return errors.New("Cannot create an AWS awsSession object: " + err.Error())
	}

	log.Println("Let's encrypt instance " + instanceID)
	ec2, instanceError := instance.New(awsSession, instanceID)
	if instanceError != nil {
		return instanceError
	}

	if !ec2.IsStopped() || !ec2.IsSupportsEncryptedVolumes() {
		return errors.New("instance must be stopped and compatible with EBS encryption")
	}

	actionDone := false
	for _, ebsVolume := range ec2.GetEBSVolumes() {
		log.Println("-- Beginning work on volume " + *ebsVolume.Ebs.VolumeId)

		sourceVolume, volumeError := volume.New(awsSession, *ebsVolume.Ebs.VolumeId, kmsKeyAlias)
		if volumeError != nil {
			return errors.New("Problem with volume initialization: " + volumeError.Error())
		}

		if sourceVolume.IsEncrypted() {
			log.Println("-- This volume is already encrypted nothing to do with this one")
			continue
		}

		encryptedVolume, encryptedVolumeError := sourceVolume.EncryptVolume()
		if encryptedVolumeError != nil {
			log.Printf("Problem while encrypting volume: %s (%s)\n", *ebsVolume.Ebs.VolumeId, encryptedVolumeError.Error())
			continue
		}

		swappingError := ec2.SwapBlockDevice(ebsVolume, encryptedVolume)
		if swappingError != nil {
			log.Println("Problem while trying to swap volumes: " + swappingError.Error())
			continue
		}

		if sourceDiscard {
			_ = sourceVolume.DeleteVolume()
		}
		actionDone = true
	}

	if actionDone {
		log.Println("Let's starts instance " + instanceID)
		startError := ec2.StartInstance()
		if startError != nil {
			return startError
		}
	}

	return nil

}
