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
	"os"

	"github.com/jbrt/ec2cryptomatic/algorithm"
	"github.com/spf13/cobra"
)

var (
	instance string
	kmsKey   string
	region   string
)

// runCmd represents the run command
var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Encrypt all EBS volumes for the given instances",

	Run: func(cmd *cobra.Command, args []string) {

		instanceID, _ := cmd.Flags().GetString("instance")
		kms, _ := cmd.Flags().GetString("kmskey")
		region, _ := cmd.Flags().GetString("region")
		discard, _ := cmd.Flags().GetBool("discard")

		error := algorithm.EncryptInstance(instanceID, region, kms, discard)
		if error != nil {
			fmt.Println("/!\\ " + error.Error())
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(runCmd)

	runCmd.Flags().StringVarP(&instance, "instance", "i", "", "Instance ID of instance of encrypt (required)")
	runCmd.Flags().StringVarP(&kmsKey, "kmskey", "k", "alias/aws/ebs", "KMS key alias name")
	runCmd.Flags().StringVarP(&region, "region", "r", "", "AWS region (required)")
	runCmd.Flags().BoolP("discard", "d", false, "Discard source volumes after encryption process (default: false)")
	runCmd.MarkFlagRequired("instance")
	runCmd.MarkFlagRequired("region")

}
