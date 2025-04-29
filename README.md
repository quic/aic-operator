# cloud_ai_openshift_operator
The AIC Operator enables the Qualcomm® Cloud AI series of hardware on OpenShift clusters by
automating the configuration and installation of their Linux device drivers and setting up
its Device Plugin.

## Getting Started
This Operator relies on the Node Feature Discovery (NFD) and Kernel Module Management
(KMM) Operators. Be sure to install them from the OperatorHub (provided by Red Hat, not
the Community).

NFD operator needs the default NodeFeatureDiscovery Custom Resource(CR) to be created after it's installed.

KMM requires configuration so that firmware can be located correctly. The following
command should work for most clusters, but make sure to check that the
'controler_config.yaml' section matches the existing configuration (note: ordering of the
elements shouldn't matter (so long as they're under the correct heading (e.g. 'webhook',
'worker', etc.)), but their existence does).

```sh
oc patch configmap kmm-operator-manager-config -n openshift-kmm --type='json' -p='[{"op": "add", "path": "/data/controller_config.yaml", "value": "healthProbeBindAddress: :8081\nmetricsBindAddress: 127.0.0.1:8080\nleaderElection:\n enabled: true\n resourceID: kmm.sigs.x-k8s.io\nwebhook:\n disableHTTP2: true\n port: 9443\nmetrics:\n enableAuthnAuthz: true\n disableHTTP2: true\n bindAddress: 0.0.0.0:8443\n secureServing: true\nworker:\n runAsUser: 0\n seLinuxType: spc_t\n firmwareHostPath: /var/lib/firmware"}]'
```

The important part added in the above config patch is
"\n setFirmwareClassPath: /var/lib/firmware".
Due to the structure of the kmm-operator-manager-config configmap, that can't be added on
its own.

**Check that the existing configuration matches outside the firmware path:**

```sh
oc get configmap kmm-operator-manager-config -n openshift-kmm -o=json
```

**After updating the KMM configuration, be sure to restart the KMM controller:**

```sh
oc delete pod -n openshift-kmm -l app.kubernetes.io/component=kmm
```

Now, on with building and deploying the AIC Operator.

### Prerequisites
- go version v1.23.0+
- docker version 17.03+.
- Access to a Kubernetes v1.31+ cluster.

### To Deploy on the cluster
**Build and push your image to the location specified by `IMG`:**

```sh
make docker-build docker-push IMG=<some-registry>/cloud_ai_openshift_operator:tag VERSION=<version>
```
### Build Operator-Bundle
```sh
make bundle-build bundle-push IMG=<some-registry>/cloud_ai_openshift_operator:tag BUNDLE_IMG=<some-registry>/cloud_ai_openshift_operator_bundle:tag VERSION=<version>
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
make deploy IMG=<some-registry>/cloud_ai_openshift_operator:tag
```

> **NOTE**: If you encounter RBAC errors, you may need to grant yourself cluster-admin
privileges or be logged in as admin.

**Create instances of AIC**
You can create the samples (examples) from the config/sample:

```sh
oc create -k config/samples/
```


### To Uninstall
**Delete the instances (CRs) of AIC from the cluster:**

```sh
oc delete -k config/samples/
```

**Delete the APIs(CRDs) from the cluster:**

```sh
make uninstall
```

**UnDeploy the controller from the cluster:**

```sh
make undeploy
```

## Contributing
For more detailed info on contributions see the [CONTRIBUTING](CONTRIBUTING.md) file.

**NOTE:** Run `make help` for more information on all potential `make` targets

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html)

## License

AIC Operator is licensed under the terms of the [LICENSE](LICENSE) file.
