package algorithm

import (
	"errors"
	"log"

	"github.com/jbrt/ec2cryptomatic/internal/ec2instance"
)

// EncryptInstance will takes an instanceID and encrypt all the related EBS volumes
func EncryptInstance(ec2 *ec2instance.Ec2Instance, kmsKeyAlias string, sourceDiscard bool, startInstance bool) error {

	if !ec2.IsStopped() || !ec2.IsSupportsEncryptedVolumes() {
		return errors.New("instance must be stopped and compatible with EBS encryption")
	}

	actionDone := false
	for _, ebsVolume := range ec2.GetEBSMappedVolumes() {
		log.Println("-- Beginning work on EBS volume " + *ebsVolume.Ebs.VolumeId)

		sourceVolume, volumeError := ec2.GetEBSVolume(*ebsVolume.Ebs.VolumeId)
		if volumeError != nil {
			return errors.New("Problem with volume initialization: " + volumeError.Error())
		}

		if sourceVolume.IsEncrypted() {
			log.Println("-- This volume is already encrypted nothing to do with this one")
			continue
		}

		encryptedVolume, encryptedVolumeError := sourceVolume.EncryptVolume(kmsKeyAlias)
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
		if startInstance {
			log.Println("Let's starts instance " + *ec2.InstanceID)
			startError := ec2.StartInstance()
			if startError != nil {
				return startError
			}
		}
	}

	return nil
}
