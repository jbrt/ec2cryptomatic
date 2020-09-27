# EC2Cryptomatic

[![Github Action](https://github.com/jbrt/ec2cryptomatic/workflows/publish-docker-image/badge.svg)](https://github.com/jbrt/ec2cryptomatic/actions?workflow=publish-docker-image)
![Docker Pulls](https://img.shields.io/docker/pulls/jbrt/ec2cryptomatic.svg?label=pulls&logo=docker)

Encrypt EBS volumes from AWS EC2 instances

**A serverless version of this script exists here:** https://github.com/jbrt/ec2cryptomatic-serverless

## Description

This tool let you :
- Encrypt all the EBS volumes for an instance
- If volumes already encrypted, re-encrypt these with the given key
- Duplicate all the source tags to the target
- Apply DeleteOnTermination flag if needs
- Preserve the original volume or not as an option (thank to @cobaltjacket)
- Start each instance after encrypting is complete (thank to @dshah22)

For your information, the workflow used to encrypt an EBS volume is:
- Take a snapshot from the original volume
- Create a new volume encrypted from that snapshot
- Swap volumes
- Delete source unencrypted volumes (if requested)

## Note about version 2.x

Since version 1, EC2Cryptomatic was coded in Python. This version 2 is a 
complete rewriting of this tool in Golang.

Why Golang instead of Python ? Principally because of fun and for training for 
the author on that language.

Golang is also a good option for a CLI tool like this (more portable than 
Python).

Python version is still available at git tag 1.2.4.

## Prerequisites

EC2Cryptomatic needs the following IAM rights:

```json
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Sid": "Stmt1504425390448",
            "Action": [
                "ec2:AttachVolume",
                "ec2:CopyImage",
                "ec2:CopySnapshot",
                "ec2:CreateSnapshot",
                "ec2:CreateVolume",
                "ec2:CreateTags",
                "ec2:DeleteSnapshot",
                "ec2:DeleteVolume",
                "ec2:DescribeInstances",
                "ec2:DescribeSnapshots",
                "ec2:DescribeVolumes",
                "ec2:DetachVolume",
                "ec2:ModifyInstanceAttribute",
                "ec2:StartInstances"
            ],
            "Effect": "Allow",
            "Resource": "*"
        }
    ]
}

```

## Syntax

Here is the syntax of ec2cryptomatic. You have to specify a AWS region name
and one EC2 instance ID.

```
Encrypt all EBS volumes for the given instances

Usage:
  ec2cryptomatic run [flags]

Flags:
  -d, --discard           Discard source volumes after encryption process (default: false)
  -h, --help              help for run
  -i, --instance string   Instance ID of instance of encrypt (required)
  -k, --kmskey string     KMS key alias name (default "alias/aws/ebs")
  -r, --region string     AWS region (required)
```

## Docker

You can build a Docker image of that tool with the Dockerfile provided in 
this repository :

`docker build -t ec2cryptomatic:latest .`

Or you can use the image already pulled into the official Docker Hub:

`docker pull jbrt/ec2cryptomatic`

## Example

![example](ec2cryptomatic.png)

## License

This project is under GPL3 license
