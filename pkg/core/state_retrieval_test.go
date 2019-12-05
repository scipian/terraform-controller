package core

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	terraformv1 "github.com/scipian/terraform-controller/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("RetrieveState", func() {
	ws := terraformv1.Workspace{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Namespace: "default",
		},
	}
	s3Bucket := os.Getenv("SCIPIAN_STATE_BUCKET")
	accessKey := os.Getenv("AWS_ACCESS_KEY_ID")
	secretKey := os.Getenv("AWS_SECRET_ACCESS_KEY")
	workspace := &ws
	filePath := fmt.Sprintf("%s/%s/%s", workspace.Namespace, workspace.Name, TFStateFileName)
	directoryPath := fmt.Sprintf("%s/%s", workspace.Namespace, workspace.Name)

	Describe("Retrieve tfstate", func() {
		Context("Retrieve tfstate", func() {
			RetrieveState(workspace, accessKey, secretKey)
			Context("Set AWS Creds", func() {
				It("Sets AWS creds", func() {
					err := setAWSCreds(accessKey, secretKey)
					Expect(err).ToNot(HaveOccurred())
					Expect(os.Getenv("AWS_ACCESS_KEY_ID")).To(Equal(accessKey))
					Expect(os.Getenv("AWS_SECRET_ACCESS_KEY")).To(Equal(secretKey))
				})
			})

			Context("Pull State", func() {
				It("Pulls tfstate from S3", func() {
					client, err := customClientWithCertPool()
					Expect(err).ToNot(HaveOccurred())
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						w.WriteHeader(http.StatusOK)
						io.WriteString(w, "Test data")
					}))

					sess := session.Must(session.NewSession(&aws.Config{
						DisableSSL:       aws.Bool(true),
						Endpoint:         aws.String(server.URL),
						Region:           aws.String("test-region"),
						S3ForcePathStyle: aws.Bool(true),
						HTTPClient:       client,
					}))
					downloader := s3manager.NewDownloader(sess)
					err = s3Puller(s3Bucket, filePath, downloader, directoryPath)
					Expect(err).ToNot(HaveOccurred())
				})

			})
			Context("Read state", func() {
				It("Reads the tfstate file", func() {
					state, err := getState(filePath)

					Expect(err).ToNot(HaveOccurred())
					Expect(state).ToNot(BeNil())
					os.RemoveAll(directoryPath)
					os.Remove(fmt.Sprintf("./%s", workspace.Namespace))

				})
			})
		})
	})

})
