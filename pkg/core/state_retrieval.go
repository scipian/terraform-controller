package core

import (
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/certifi/gocertifi"
	terraformv1 "github.com/scipian/terraform-controller/api/v1"
)

//RetrieveState function downloads tfstate file from S3 bucket and returns the processed tfstate as a string
func RetrieveState(workspace *terraformv1.Workspace, accessKey string, secretKey string) (string, error) {

	s3Bucket, set := os.LookupEnv("SCIPIAN_STATE_BUCKET")
	if !set {
		return "", fmt.Errorf("Error: Env variable SCIPIAN_STATE_BUCKET not set")
	}

	filePath := fmt.Sprintf("%s/%s/%s", workspace.Namespace, workspace.Name, TFStateFileName)
	directoryPath := fmt.Sprintf("%s/%s", workspace.Namespace, workspace.Name)

	if err := setAWSCreds(accessKey, secretKey); err != nil {
		return "", err
	}

	pullerSession, err := createNewSession()
	if err != nil {
		return "", err
	}

	// Pull state from Scipian S3 backend
	downloader := s3manager.NewDownloader(pullerSession)
	err = s3Puller(s3Bucket, filePath, downloader, directoryPath)
	if err != nil {
		return "", fmt.Errorf("Error: %v", err)
	}

	//Read the tfstate file
	state, err := getState(filePath)
	if err != nil {
		return "", err
	}
	return state, nil
}

// setAWSCreds sets the appropriate environment variables that AWS SDK expects
func setAWSCreds(awsAccessKey string, awsSecretKey string) error {

	if err := os.Setenv("AWS_ACCESS_KEY_ID", awsAccessKey); err != nil {
		return err
	}
	if err := os.Setenv("AWS_SECRET_ACCESS_KEY", awsSecretKey); err != nil {
		return err
	}
	return nil

}

// s3Puller pulls a terraform.tfstate file from an S3 bucket
func s3Puller(s3Bucket string, filePath string, downloader *s3manager.Downloader, directoryPath string) error {

	if _, err := os.Stat(directoryPath); os.IsNotExist(err) {
		err := os.MkdirAll(directoryPath, 0755)
		if err != nil {
			return fmt.Errorf("Failed to create tfstate download directory: %v", err)
		}
	}
	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		os.Remove(filePath)
	}
	stateFD, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("Failed to create terraform.tfstate file: %v ", err)
	}

	_, err = downloader.Download(stateFD, &s3.GetObjectInput{
		Bucket: aws.String(s3Bucket),
		Key:    aws.String(filePath),
	})
	if err != nil {
		return fmt.Errorf("failed to download state: %v ", err)
	}

	return nil
}

// customClientWithCertPool creates a cert pool to be used with the http client
func customClientWithCertPool() (*http.Client, error) {
	certPool, err := gocertifi.CACerts()
	if err != nil {
		return nil, fmt.Errorf("failed to build cert pool: %v", err)
	}

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{RootCAs: certPool},
	}

	client := &http.Client{Transport: transport}

	return client, nil
}

//createNewSession creates a new AWS session for secured communication between client and server
func createNewSession() (*session.Session, error) {
	client, err := customClientWithCertPool()
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %v", err)
	}

	sess, err := session.NewSession(&aws.Config{
		Region:     aws.String("us-west-2"),
		HTTPClient: client,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create new session: %v ", err)
	}

	return sess, nil
}

//getState processes the downloaded terraform state file
func getState(filePath string) (string, error) {
	jsonFile, err := os.Open(filePath)
	if err != nil {
		fmt.Println(err)
		return "", err
	}
	defer jsonFile.Close()
	byteValue, _ := ioutil.ReadAll(jsonFile)
	return (string(byteValue)), nil
}

//CustomClientWithCertPool is a public function that creates a cert pool to be used with the http client
func CustomClientWithCertPool() (*http.Client, error) {
	client, err := customClientWithCertPool()
	return client, err
}

//S3Puller is a public function that pulls a terraform.tfstate file from an S3 bucket
func S3Puller(s3Bucket string, filePath string, downloader *s3manager.Downloader, directoryPath string) error {
	return s3Puller(s3Bucket, filePath, downloader, directoryPath)
}
