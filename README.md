Terraform Controller
====================

Introduction
------------

This is a custom Kubernetes controller designed to run in the Scipian 
namespace. It watches for changes on incoming Custom Resources and launches Jobs 
to create/update/destroy infrastructure using Terraform.

It is built with [Kubebuilder 2.0][kubebuilder], with full documentation found 
[here][kubebuilder-book].

[kubebuilder]: https://github.com/kubernetes-sigs/kubebuilder
[kubebuilder-book]: https://book.kubebuilder.io/

Setting Up the Cluster
----------------------

The Scipian Terraform Controller expects a few things to be set up in the
cluster it will run in:

1. A `scipian` namespace
1. A secret named `scipian-aws-iam-creds` with AWS IAM secret accesss key and
access key ID as `aws_access_key_id` and `aws_secret_access_key` respectively.
These creds are for Scipian's S3 bucket where it will access Terraform State,
and should be for that AWS account. *NOTE*: These should be base64 encrypted.
In order to avoid new line characters in the base64 encrypted string, use the
following flags when encrypting: `echo -n <aws_cred> | base64 -w 0`.
1. An S3 bucket and corresponding DynamoDB table. Set these in
`config/manager/manager.yaml` in the ConfigMap section. *NOTE*: The DynamoDB
table should have the same name as the S3 bucket, but with `-locking` appended
to it.
1. `make install` - installs Custom Resource Definitions (CRDs) into the cluster

Running Locally
---------------

To run the project locally for developing:

1. Using [Direnv][direnv], set up your `.envrc` file with `SCIPIAN_STATE_BUCKET`
and `SCIPIAN_STATE_LOCKING` pointing to your desired s3 bucket and 
DynamoDB table respectively.
1. `go get`
1. `make install`
1. `make run` (this will run against the cluster defined in `$HOME/.kube/config`)

[direnv]: https://direnv.net/

Deploying in Cluster
------------------

To deploy the controller in a cluster:

1. `make docker-build`
1. `make docker-push`
1. `make deploy`


Testing
-------

This project uses [Ginkgo][ginkgo] as a BDD testing framework. Make sure to
have Ginkgo installed locally.

[ginkgo]: https://onsi.github.io/ginkgo/
