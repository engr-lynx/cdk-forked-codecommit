package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/go-git/go-git/v5/plumbing/transport/http"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/iam"
)

func handler(ctx context.Context) (string, error) {

	srcRepo := os.Getenv("SRC_REPO")
	destRepo := os.Getenv("DEST_REPO")
	data := fmt.Sprintf("SrcRepo: %v DestRepo: %v", srcRepo, destRepo)
	log.Printf(data)

	sess, err := session.NewSession()
	if err != nil {
		return data, fmt.Errorf("error creating session: %s", err.Error())
	}
	svc := iam.New(sess)

	// ToDo: can this be generated?
	gitUserName := "git-user"
	_, err = svc.CreateUser(&iam.CreateUserInput{
		UserName: aws.String(gitUserName),
	})
	if err != nil {
		return data, fmt.Errorf("error creating git user: %s", err.Error())
	}

	codecommitPowerUserArn := "arn:aws:iam::aws:policy/AWSCodeCommitPowerUser"
	_, err = svc.AttachUserPolicy(&iam.AttachUserPolicyInput{
		PolicyArn: aws.String(codecommitPowerUserArn),
		UserName: aws.String(gitUserName),
	})
	if err != nil {
		return data, fmt.Errorf("error attaching permission to git user: %s", err.Error())
	}

	codecommitPrincipal := "codecommit.amazonaws.com"
	credentials, err := svc.CreateServiceSpecificCredential(&iam.CreateServiceSpecificCredentialInput{
		ServiceName: aws.String(codecommitPrincipal),
		UserName: aws.String(gitUserName),
	})
	if err != nil {
		return data, fmt.Errorf("error creating git credentials: %s", err.Error())
	}

	auth := &http.BasicAuth{
		Username: aws.StringValue(credentials.ServiceSpecificCredential.ServiceUserName),
		Password: aws.StringValue(credentials.ServiceSpecificCredential.ServicePassword),
	}

	repo, err := git.Clone(memory.NewStorage(), nil, &git.CloneOptions{
		URL:      srcRepo,
		Progress: os.Stdout,
	})
	if err != nil {
		return data, fmt.Errorf("clone of source repo failed: %s", err.Error())
	}

	remote, err := repo.CreateRemote(&config.RemoteConfig{
		Name: "aws",
		URLs: []string{destRepo},
	})
	if err != nil {
		return data, fmt.Errorf("create remote failed: %s", err.Error())
	}

	for i := 0; i < 25; i++ {
		err = remote.Push(&git.PushOptions{
			RemoteName: "aws",
			Auth:       auth,
		})
		if err == nil || err.Error() != "authorization failed" {
			break
		}
		time.Sleep(5 * time.Second)
	}
	if err != nil {
		return data, fmt.Errorf("error pushing to CodeCommit repo: %s", err.Error())
	}

	_, err = svc.DeleteServiceSpecificCredential(&iam.DeleteServiceSpecificCredentialInput{
		ServiceSpecificCredentialId: credentials.ServiceSpecificCredential.ServiceSpecificCredentialId,
		UserName: aws.String(gitUserName),
	})
	if err != nil {
		return data, fmt.Errorf("error deleting git credentials: %s", err.Error())
	}

	_, err = svc.DetachUserPolicy(&iam.DetachUserPolicyInput{
		PolicyArn: aws.String(codecommitPowerUserArn),
		UserName: aws.String(gitUserName),
	})
	if err != nil {
		return data, fmt.Errorf("error detaching permission from git user: %s", err.Error())
	}

	_, err = svc.DeleteUser(&iam.DeleteUserInput{
		UserName: aws.String(gitUserName),
	})
	if err != nil {
		return data, fmt.Errorf("error deleting git user: %s", err.Error())
	}

	return data, nil
}

func main() {
	lambda.Start(handler)
}
