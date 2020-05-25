package algorithm

import (
	"errors"
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/jbrt/ec2cryptomatic/instance"
	"github.com/jbrt/ec2cryptomatic/volume"
)

// EncryptInstance will takes an instanceID and encrypt all the related EBS volumes
func EncryptInstance(instanceID, region, kmsKeyAlias string, sourceDiscard bool) error {
	fmt.Print("\t\t-=[ EC2Cryptomatic ]=-\n")

	session, err := session.NewSession(&aws.Config{Region: aws.String(region)})
	if err != nil {
		return errors.New("Cannot create an AWS session object: " + err.Error())
	}

	log.Println("Let's encrypt instance " + instanceID)
	ec2, instanceError := instance.New(session, instanceID)
	if instanceError != nil {
		return instanceError
	}

	if !ec2.IsStopped() || !ec2.IsSupportsEncryptedVolumes() {
		return errors.New("Instance must be stopped and compatible with EBS encryption")
	}

	for _, ebsVolume := range ec2.GetEBSVolumes() {
		log.Println("-- Beginning work on volume " + *ebsVolume.Ebs.VolumeId)

		sourceVolume, volumeError := volume.New(session, *ebsVolume.Ebs.VolumeId, "alias/aws/ebs")
		if volumeError != nil {
			return errors.New("Problem with volume initialization: " + volumeError.Error())
		}

		if sourceVolume.IsEncrypted() {
			log.Println("-- This volume is already encrypted nothing to do with this one")
			continue
		}

		encryptedVolume, encryptedVolumeError := sourceVolume.EncryptVolume()
		if encryptedVolumeError != nil {
			log.Fatalln("Problem while encrypting volume: %s (%s)", *ebsVolume.Ebs.VolumeId, encryptedVolumeError.Error())
			continue
		}

		swappingError := ec2.SwapBlockDevice(ebsVolume, encryptedVolume)
		if swappingError != nil {
			log.Fatalln("Problem while trying to swap volumes: " + swappingError.Error())
			continue
		}

		if sourceDiscard {
			sourceVolume.DeleteVolume()
		}

	}

	log.Println("Let's starts instance " + instanceID)
	startError := ec2.StartInstance()
	if startError != nil {
		return startError
	}
	return nil

}
