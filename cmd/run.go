/*
Copyright © 2020 Julien B.

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
*/
package cmd

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"log"


	"github.com/jbrt/ec2cryptomatic/internal/algorithm"
	"github.com/jbrt/ec2cryptomatic/internal/ec2instance"
	"github.com/spf13/cobra"
)

// runCmd represents the run command
var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Encrypt all EBS volumes for the given instances",

	Run: func(cmd *cobra.Command, args []string) {

		instanceID, _ := cmd.Flags().GetString("instance")
		kms, _ := cmd.Flags().GetString("kmsKeyAlias")
		region, _ := cmd.Flags().GetString("region")
		discard, _ := cmd.Flags().GetBool("discard")
		startInstance, _ := cmd.Flags().GetBool("start")

		fmt.Print("\t\t-=[ EC2Cryptomatic ]=-\n")

		awsSession, err := session.NewSession(&aws.Config{Region: aws.String(region)})
		if err != nil {
			log.Fatalln("Cannot create an AWS awsSession object: " + err.Error())
		}

		ec2Instance, instanceError := ec2instance.New(awsSession, instanceID)
		if instanceError != nil {
			log.Fatalln(instanceError)
		}

		errorAlgorithm := algorithm.EncryptInstance(ec2Instance, kms, discard, startInstance)
		if errorAlgorithm != nil {
			log.Fatalln("/!\\ " + errorAlgorithm.Error())
		}
	},
}

func init() {
	var awsRegion, instanceID, kmsKeyAlias string

	rootCmd.AddCommand(runCmd)

	runCmd.Flags().StringVarP(&instanceID, "instance", "i", "", "Instance ID of instance of encrypt (required)")
	runCmd.Flags().StringVarP(&kmsKeyAlias, "kmsKeyAlias", "k", "alias/aws/ebs", "KMS key alias name")
	runCmd.Flags().StringVarP(&awsRegion, "region", "r", "", "AWS region (required)")
	runCmd.Flags().BoolP("discard", "d", false, "Discard source volumes after encryption process (default: false)")
	runCmd.Flags().BoolP("start", "s", false, "Start instance after volume encryption (default: false)")
	_ = runCmd.MarkFlagRequired("instance")
	_ = runCmd.MarkFlagRequired("region")
}
