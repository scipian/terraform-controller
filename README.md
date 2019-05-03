Terraform Controller
====================

Introduction
------------

This is a custom Kubernetes controller designed to run in the kube-system 
namespace. It watches for changes on incoming CRD's and launches Jobs to 
create/update/destroy infrastructure using Terraform.

It is built with [Kubebuilder][kubebuilder], with full documentation found 
[here][kubebuilder-book].

[kubebuilder]: https://github.com/kubernetes-sigs/kubebuilder
[kubebuilder-book]: https://book.kubebuilder.io/

Running Locally
---------------

To run the project locally for developing:

1. dep ensure
1. using [Direnv][direnv], set up your `.envrc` file with `SCIPIAN_STATE_BUCKET`
and `SCIPIAN_STATE_LOCKING` pointing to your s3 bucket and DynamoDB table
respectively.
1. make
1. make install
1. make run (must have kubeconfig for our cluster in .kube/config)

[direnv]: https://direnv.net/
