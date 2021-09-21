package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"
	"encoding/json"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/go-git/go-git/v5/plumbing/transport/http"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	secret "github.com/aws/aws-sdk-go/service/secretsmanager"
	"github.com/aws/aws-sdk-go/service/iam"
)

type Creds struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func handler(ctx context.Context) (string, error) {

	srcRepo := os.Getenv("SRC_REPO")
	destRepo := os.Getenv("DEST_REPO")
	// destRepoArn := os.Getenv("DEST_REPO_ARN")
	destSecret := os.Getenv("DEST_SECRET")

	data := fmt.Sprintf("SrcRepo: %v DestRepo: %v DestSecret: %v", srcRepo, destRepo, destSecret)
	log.Printf(data)

	sess1, err := session.NewSession()
	if err != nil {
		return data, fmt.Errorf("error creating session: %s", err.Error())
	}

	svc1 := secret.New(sess1)
	credsValue, err := svc1.GetSecretValue(&secret.GetSecretValueInput{
		SecretId: aws.String(destSecret),
	})
	if err != nil {
		return data, fmt.Errorf("error getting secret value: %s", err.Error())
	}

	var creds Creds
	err = json.Unmarshal([]byte(*credsValue.SecretString), &creds)
	if err != nil {
		return data, fmt.Errorf("error unmarshaling secret string: %s", err.Error())
	}
	log.Printf(creds.Username)
	log.Printf(creds.Password)

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

	sess, err := session.NewSession()
	if err != nil {
		return data, fmt.Errorf("error creating session: %s", err.Error())
	}
	svc := iam.New(sess)

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

	for attached := false; !attached; {
		time.Sleep(3 * time.Second)
		log.Printf("checking policies...")
		policies, err := svc.ListAttachedUserPolicies(&iam.ListAttachedUserPoliciesInput{
			UserName: aws.String(gitUserName),
		})
		if err != nil {
			return data, fmt.Errorf("error listing git user policies: %s", err.Error())
		}
		for _, policy := range policies.AttachedPolicies {
			log.Printf(aws.StringValue(policy.PolicyArn))
			attached = aws.StringValue(policy.PolicyArn) == codecommitPowerUserArn
			if attached {
				break
			}
		}
	}

	codecommitPrincipal := "codecommit.amazonaws.com"
	credentials, err := svc.CreateServiceSpecificCredential(&iam.CreateServiceSpecificCredentialInput{
		ServiceName: aws.String(codecommitPrincipal),
		UserName: aws.String(gitUserName),
	})
	if err != nil {
		return data, fmt.Errorf("error creating git credentials: %s", err.Error())
	}

	for active := false; !active; {
		time.Sleep(3 * time.Second)
		log.Printf("checking credentials...")
		serviceCreds, err := svc.ListServiceSpecificCredentials(&iam.ListServiceSpecificCredentialsInput{
			UserName: aws.String(gitUserName),
		})
		if err != nil {
			return data, fmt.Errorf("error listing git user credentials: %s", err.Error())
		}
		for _, credential := range serviceCreds.ServiceSpecificCredentials {
			log.Printf(aws.StringValue(credential.ServiceName))
			log.Printf(aws.StringValue(credential.Status))
			active = aws.StringValue(credential.ServiceName) == codecommitPrincipal && aws.StringValue(credential.Status) == "Active"
			if active {
				break
			}
		}
	}

	auth := &http.BasicAuth{
		Username: aws.StringValue(credentials.ServiceSpecificCredential.ServiceUserName),
		Password: aws.StringValue(credentials.ServiceSpecificCredential.ServicePassword),
	}

	log.Printf("2")
	log.Printf(auth.Username)
	log.Printf(auth.Password)

	err = remote.Push(&git.PushOptions{
		RemoteName: "aws",
		Auth:       auth,
	})
	if err != nil {
		return data, fmt.Errorf("push to destination repo failed: %s", err.Error())
	}

	log.Printf("3")
	log.Printf(auth.Username)
	log.Printf(auth.Password)

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
