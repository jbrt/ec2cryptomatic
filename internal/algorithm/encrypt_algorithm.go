package algorithm

import (
	"errors"
	"log"

	"github.com/jbrt/ec2cryptomatic/internal/ec2instance"
)

var	nonEligibleForEncryptionError = errors.New("instance must be stopped and compatible with EBS encryption")

// EncryptInstance will takes an instanceID and encrypt all the related EBS volumes
func EncryptInstance(ec2 *ec2instance.Ec2Instance, kmsKeyAlias string, discardSource bool, startInstance bool) error {

	if !ec2.IsStopped() || !ec2.IsSupportsEncryptedVolumes() {
		return nonEligibleForEncryptionError
	}

	actionDone := false
	for _, ebsVolume := range ec2.GetEBSMappedVolumes() {
		log.Println("-- Beginning work on EBS volume " + *ebsVolume.Ebs.VolumeId)

		sourceVolume, volumeError := ec2.GetEBSVolume(*ebsVolume.Ebs.VolumeId); if volumeError != nil {
			return errors.New("Problem with volume initialization: " + volumeError.Error())
		}

		encryptedVolume, encryptedVolumeError := sourceVolume.EncryptVolume(kmsKeyAlias); if encryptedVolumeError != nil {
			log.Printf("Problem while encrypting volume: %s (%s)\n", *ebsVolume.Ebs.VolumeId, encryptedVolumeError.Error())
			continue
		}

		if swappingError := ec2.SwapBlockDevice(ebsVolume, encryptedVolume); swappingError != nil {
			log.Println("Problem while trying to swap volumes: " + swappingError.Error())
			continue
		}

		if discardSource {
			_ = sourceVolume.DeleteVolume()
		}
		actionDone = true
	}

	if actionDone && startInstance {
		log.Println("Let's starts instance " + *ec2.InstanceID)
		if startError := ec2.StartInstance(); startError != nil {
			return startError
		}
	}

	return nil
}
