# k8s-auth-operator
Kubernetes auth operator is a project that aims to provide a solution to manage the authentication of the Kubernetes cluster. It is built using the Kubebuilder framework.


## Description
As the Kubernetes cluster grows, managing the authentication of the cluster becomes a tedious task. This project aims to provide a solution to manage the authentication of the Kubernetes cluster. The operator will manage the authentication of the cluster by creating and managing the service accounts, roles, and role bindings. The operator will also provide a way to manage the authentication of the cluster using a declarative approach.<br>
It uses a custom resource definition (CRD) to define the authentication configuration of the cluster. The operator will watch for changes to the CRD and reconcile the state of the cluster based on the configuration defined in the CRD.
<br>The Provided CRDs are:
- **Context:** This CRD defines the context of the authentication configuration. It groups the list of the relevant namespaces by name or by regex.
- **Role:** This CRD defines the role of the authentication configuration. It defines the role name and the list of the resources that the role can access.
- **User:** This CRD defines the user of the authentication configuration. It defines the username and the list of the roles that the user can access.

## Getting Started

### Prerequisites
- go version v1.22.0+
- docker version 17.03+.
- kubectl version v1.11.3+.
- Access to a Kubernetes v1.11.3+ cluster.

### To Deploy on the cluster
**Build and push your image to the location specified by `IMG`:**

```sh
make docker-build docker-push IMG=ghcr.io/Dhouib-Mohamed/k8s-auth-operator:tag
```

**NOTE:** This image ought to be published in the personal registry you specified.
And it is required to have access to pull the image from the working environment.
Make sure you have the proper permission to the registry if the above commands don’t work.

**Install the CRDs into the cluster:**

```sh
make install
```

**Deploy the Manager to the cluster with the image specified by `IMG`:**

```sh
make deploy IMG=ghcr.io/Dhouib-Mohamed/k8s-auth-operator:tag
```

> **NOTE**: If you encounter RBAC errors, you may need to grant yourself cluster-admin
privileges or be logged in as admin.

**Create instances of your solution**
You can apply the samples (examples) from the config/sample:

```sh
kubectl apply -k config/samples/
```

>**NOTE**: Ensure that the samples has default values to test it out.

### To Uninstall
**Delete the instances (CRs) from the cluster:**

```sh
kubectl delete -k config/samples/
```

**Delete the APIs(CRDs) from the cluster:**

```sh
make uninstall
```

**UnDeploy the controller from the cluster:**

```sh
make undeploy
```

## Project Distribution

Following are the steps to build the installer and distribute this project to users.

1. Build the installer for the image built and published in the registry:

```sh
make build-installer IMG=ghcr.io/Dhouib-Mohamed/k8s-auth-operator:tag
```

NOTE: The makefile target mentioned above generates an 'install.yaml'
file in the dist directory. This file contains all the resources built
with Kustomize, which are necessary to install this project without
its dependencies.

2. Using the installer

Users can just run kubectl apply -f <URL for YAML BUNDLE> to install the project, i.e.:

```sh
kubectl apply -f https://raw.githubusercontent.com/Dhouib-Mohamed/k8s-auth-operator/<tag>/dist/install.yaml
```

## Contributing
In order to contribute to this project, please follow the following steps:

1. Fork the repository
2. Create a new branch (`git checkout -b feature`)
3. Make the appropriate changes in the files
4. Add changes to reflect the changes made
5. Commit your changes (`git commit -am 'Add new feature'`)
6. Push to the branch (`git push origin feature`)
7. Create a Pull Request
8. Get the PR approved



**NOTE:** Run `make help` for more information on all potential `make` targets

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html)

## License

Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

