
## POC: deploy and test standalone-trustd with kamaji

Deploy TCP

```bash
k create -f manifests/1.yaml
```

Verify TCP is running

```bash
# k get tcp
NAME                VERSION   STATUS   CONTROL-PLANE ENDPOINT   KUBECONFIG                           DATASTORE     AGE
kubernetes-foo      v1.33.0   Ready    10.96.100.2:6443         kubernetes-foo-admin-kubeconfig      tenant-root   36m
```

Check that the Service has been created

Example output:

```console
NAME             TYPE        CLUSTER-IP    EXTERNAL-IP   PORT(S)                       AGE
kubernetes-foo   ClusterIP   10.96.100.2   <none>        6443/TCP,8132/TCP,50001/TCP   35m
```

Add the Service IP to certSANs in the manifest

```bash
vim manifests/1.yaml
```

```yaml
networkProfile:
  certSANs:
  - kubernetes-foo.kvaps.dev5.infra.example.org
  - 10.96.100.2
```

Apply the changes:

```bash
k apply -f manifests/1.yaml
```

Wait until Kamaji recreates the certificate.

---

Stop Kamaji

```bash
k scale --replicas 0 deployment -n cozy-kamaji kamaji
```

Apply the Deployment and Service patch

```bash
k apply -f manifests/2.yaml
```

Changes:
- The Deployment now includes a trustd sidecar.
- The Service publishes port 50001.

Deploy the Talos VM

```bash
k apply -f manifests/3.yaml
```


---

Get your kubeconfig

```bash
k get secret kubernetes-foo-admin-kubeconfig -o go-template='{{ index .data "super-admin.conf" | base64decode }}' > kubeconfig
export KUBECONFIG=$PWD/kubeconfig
```

Create a join token

```bash
/ # kubeadm token create --print-join-command
kubeadm join 10.96.100.2:6443 --token snr8ly.frwiolwcgg2updc7 --discovery-token-ca-cert-hash sha256:c95edba9ed16b4784004d15e504afa42f79d043c9669e94c747fb9fe7b736009
```

Get the base64-encoded CA certificate

```bash
k get secret kubernetes-foo-ca -o go-template='{{ index .data "tls.crt" }}'
```

Generate the Talos config

```bash
talosctl gen config kubernetes-foo https://kubernetes-foo:6443
```

Edit the Talos config and add:

```yaml
machine:
    type: worker
    token: 2k882v.z2vi7kefznukil1o # The `token` from your secret (manifests/2.yaml)
    ca:
        # ca cert from your cluster
        crt: <base64-encoded CA cert>
    certSANs:
       - 127.0.0.1 # for local debugging
    kubelet:
        nodeIP:
            validSubnets:
                - 10.0.0.0/8
cluster:
    id: null # can be disabled
    secret: null # can be disabled
    controlPlane:
        endpoint: https://kubernetes-foo:6443 # your kubernetes api endpoint
    clusterName: kubernetes-foo
    network:
        dnsDomain: cluster.local
        podSubnets:
            - 10.243.0.0/16
        # The service subnet CIDR.
        serviceSubnets:
            - 10.94.0.0/12
        
     token: snr8ly.frwiolwcgg2updc7 # The `token` from your kubeadm join command
    ca:
        # ca cert from your cluster
        crt: <base64-encoded CA cert>
    discovery:
        enabled: false # can be disabled
```

Forward the port

```bash
virtctl port-forward vmi/talos 50000:50000
```

Apply the config

```bash
talosctl apply -f worker.yaml -e 127.0.0.1 -n 127.0.0.1 -i
```

---

Generate a new talosconfig:

```bash
cat > secrets.yaml <<EOT
cluster:
    id: null
    secret: null
secrets:
    bootstraptoken: null
    secretboxencryptionsecret: null
trustdinfo:
    token: null
certs:
    etcd:
        crt: null
        key: null
    k8s:
        crt: null
        key: null
    k8saggregator:
        crt: null
        key: null
    k8sserviceaccount:
        key: null
    os:
        crt: $(kubectl get secret kubernetes-foo-ca -o go-template='{{ index .data "tls.crt" }}')
        key: $(kubectl get secret kubernetes-foo-ca -o go-template='{{ index .data "tls.key" }}')
EOT
talosctl gen config --with-secrets secrets.yaml kubernetes-foo https://kubernetes-foo:6443 -t talosconfig --force
```
