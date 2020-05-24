package constants

const (
	// VERSION is the overall version of the program
	VERSION string = "2.0.0"
	// VolumeMaxAttempts how many attempts EC2 EBS waiters will used between SDK actions (snapshot volume)
	VolumeMaxAttempts int = 10000
	// InstanceMaxAttempts how many attempts EC2 waiters will used between SDK actions (attach/detach volume)
	InstanceMaxAttempts int = 120
)
