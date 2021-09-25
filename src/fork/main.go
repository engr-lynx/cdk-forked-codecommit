package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/iam"
)

func handler(ctx context.Context) (string, error) {

	codecommitPowerUserArn := "arn:aws:iam::aws:policy/AWSCodeCommitPowerUser"
	codecommitPrincipal := "codecommit.amazonaws.com"
	remoteName := "aws"

	var errStr string
	var creds *iam.CreateServiceSpecificCredentialOutput
	var repo *git.Repository

	srcRepo := os.Getenv("SRC_REPO")
	destRepo := os.Getenv("DEST_REPO")
	userName := os.Getenv("USER_NAME")
	data := fmt.Sprintf("SrcRepo: %v DestRepo: %v UserName: %v", srcRepo, destRepo, userName)
	log.Printf(data)

	sess, err := session.NewSession()
	svc := iam.New(sess)
	if err != nil {
		errStr = fmt.Sprintf("error creating session: %s\n", err.Error())
		goto exit
	}

	_, err = svc.AttachUserPolicy(&iam.AttachUserPolicyInput{
		PolicyArn: aws.String(codecommitPowerUserArn),
		UserName: aws.String(userName),
	})
	if err != nil {
		errStr = fmt.Sprintf("error attaching permission to git user: %s\n", err.Error())
		goto exit
	}

	creds, err = svc.CreateServiceSpecificCredential(&iam.CreateServiceSpecificCredentialInput{
		ServiceName: aws.String(codecommitPrincipal),
		UserName: aws.String(userName),
	})
	if err != nil {
		errStr = fmt.Sprintf("error creating git credentials: %s\n", err.Error())
		goto detachPolicy
	}

	repo, err = git.Clone(memory.NewStorage(), nil, &git.CloneOptions{
		URL: srcRepo,
		Progress: os.Stdout,
	})
	if err != nil {
		errStr = fmt.Sprintf("clone of source repo failed: %s\n", err.Error())
		goto deleteCredential
	}

	_, err = repo.CreateRemote(&config.RemoteConfig{
		Name: remoteName,
		URLs: []string{destRepo},
	})
	if err != nil {
		errStr = fmt.Sprintf("create remote failed: %s\n", err.Error())
		goto deleteCredential
	}

	for i := 0; i < 25; i++ {
		err = repo.Push(&git.PushOptions{
			RemoteName: remoteName,
			Auth: &http.BasicAuth{
				Username: aws.StringValue(creds.ServiceSpecificCredential.ServiceUserName),
				Password: aws.StringValue(creds.ServiceSpecificCredential.ServicePassword),
			},
			Progress: os.Stdout,
		})
		if err == nil || err != transport.ErrAuthorizationFailed {
			break
		}
		log.Printf("git user not ready")
		time.Sleep(5 * time.Second)
		log.Printf("retrying")
	}
	if err != nil && err != git.NoErrAlreadyUpToDate {
		errStr = fmt.Sprintf("error pushing to CodeCommit repo: %s\n", err.Error())
		goto deleteCredential
	}

deleteCredential:
	_, err = svc.DeleteServiceSpecificCredential(&iam.DeleteServiceSpecificCredentialInput{
		ServiceSpecificCredentialId: creds.ServiceSpecificCredential.ServiceSpecificCredentialId,
		UserName: aws.String(userName),
	})
	if err != nil {
		errStr += fmt.Sprintf("error deleting git credentials: %s\n", err.Error())
	}

detachPolicy:
	_, err = svc.DetachUserPolicy(&iam.DetachUserPolicyInput{
		PolicyArn: aws.String(codecommitPowerUserArn),
		UserName: aws.String(userName),
	})
	if err != nil {
		errStr += fmt.Sprintf("error detaching permission from git user: %s\n", err.Error())
	}

exit:
	if errStr != "" {
		return data, errors.New(errStr)
	} else {
		return data, nil
	}

}

func main() {
	lambda.Start(handler)
}
