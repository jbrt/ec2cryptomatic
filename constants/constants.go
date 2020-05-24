package constants

const (
	// VERSION is the overall version of the program
	VERSION string = "2.0.0"
	// How many attempts EC2 EBS waiters will used between SDK actions (snapshot volume)
	volumeMaxAttempts int = 10000
	// How many attempts EC2 waiters will used between SDK actions (attach/detach volume)
	instanceMaxAttempts int = 120
)
