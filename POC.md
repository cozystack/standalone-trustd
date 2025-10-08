
Install TCP:

```
k create -f manifests/1.yaml
```

check that tcp is deployed:

```
# k get tcp
NAME                VERSION   STATUS   CONTROL-PLANE ENDPOINT   KUBECONFIG                           DATASTORE     AGE
kubernetes-foo      v1.33.0   Ready    10.96.100.2:6443         kubernetes-foo-admin-kubeconfig      tenant-root   36m
```


check that the service is created:

```
NAME             TYPE        CLUSTER-IP    EXTERNAL-IP   PORT(S)                       AGE
kubernetes-foo   ClusterIP   10.96.100.2   <none>        6443/TCP,8132/TCP,50001/TCP   35m
```

take service ip. and put it into certSANs in the manifest:

```
vim manifests/1.yaml
```

```
networkProfile:
  certSANs:
  - kubernetes-foo.kvaps.dev5.infra.aenix.org
  - 10.96.100.2
```

apply the change:

```
k apply -f manifests/1.yaml
```

wait until kamaji reconcile this cert

---

disable kamaji

```
k scale --replicas 0 deployment -n cozy-kamaji kamaji
```

apply patched deployment and service:
```
k apply -f manifests/2.yaml
```

changes are:
- deployment now has trustd sidecar
- service now has :50001 port

deploy talos vm:

```
k apply -f manifests/3.yaml
```

---

get your kubeconfig:

```
k get secret kubernetes-foo-admin-kubeconfig -o go-template='{{ index .data "super-admin.conf" | base64decode }}' > kubeconfig
export KUBECONFIG=$PWD/kubeconfig
```

create token for join:
```
/ # kubeadm token create --print-join-command
kubeadm join 10.96.100.2:6443 --token snr8ly.frwiolwcgg2updc7 --discovery-token-ca-cert-hash sha256:c95edba9ed16b4784004d15e504afa42f79d043c9669e94c747fb9fe7b736009
```

get your base64 encoded cert:

```
k get secret kubernetes-foo-ca -o go-template='{{ index .data "tls.crt" }}'
```


generate talos config:
```
talosctl gen config kubernetes-foo https://kubernetes-foo:6443
```

edit talos config and put there:

```yaml
machine:
    type: worker
    token: 2k882v.z2vi7kefznukil1o # The `token` from your secret (manifests/2.yaml)
    ca:
        # ca cert from your cluster
        crt: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSURCVENDQWUyZ0F3SUJBZ0lJUlpHVUtpci9DRUV3RFFZSktvWklodmNOQVFFTEJRQXdGVEVUTUJFR0ExVUUKQXhNS2EzVmlaWEp1WlhSbGN6QWVGdzB5TlRFd01EZ3hNVFEyTkROYUZ3MHpOVEV3TURZeE1UVXhORE5hTUJVeApFekFSQmdOVkJBTVRDbXQxWW1WeWJtVjBaWE13Z2dFaU1BMEdDU3FHU0liM0RRRUJBUVVBQTRJQkR3QXdnZ0VLCkFvSUJBUUNqSG56dDFsZ2U3WU96dURTeEp1SC8xTVlURzMrdGRjRGU5TFY0bUQvNU5oT1ZMS2t5b25RS0VIUTIKc3N5Nkhyc1RGcnFObFFLZzQzeVhFTHIrTENEVXJNMDhEOStUOGo1MEZ0Z0pSei9aTTFvaG1tVEtBMUYzSlUyeAphRDFKQnlnOFAwSkFqWlhiVFYyYmd2bjlvQVpVOXpzTVQ5a3Q1bFg4YTJvSWF4ZmhPVG9CbXZZNnpZdHpyZHB1Cm9ISHdXZkJIaTArUHN5NHBXaGVndjdZQWllRTJsc1BoMnF5SWRLUVkvcXp5M2JEM2hDWjg4RkZMaStzam5NV0wKRnhYWlg5NS9OWDNHWTZ3QjVQelNycHJ6WnhKbjhjRi9XT3NLWjZDbWloZDJPTDhJb1psMkpmanhQRzlPSUcydgpFbnZYQTBPMExzKzkyR3RwaVo3U21oOThYS0lUQWdNQkFBR2pXVEJYTUE0R0ExVWREd0VCL3dRRUF3SUNwREFQCkJnTlZIUk1CQWY4RUJUQURBUUgvTUIwR0ExVWREZ1FXQkJUNHpaMDA1TVFCaUVDalVjVWZ1cjAwWHVoQWlEQVYKQmdOVkhSRUVEakFNZ2dwcmRXSmxjbTVsZEdWek1BMEdDU3FHU0liM0RRRUJDd1VBQTRJQkFRQm1FVnJLQkVENwpnTWt1VlQ1S2t0czBQRERHcmt4ZWp4M0YyMXE4S2N6QTBRcGpONExueG9qeEI5STVzM2QwZTUwL0N3dW1BMkVWCmJOZEtJZ2x2bG1RMXNGQmtyQnJnRUtoY01IemlGZEVkQUgrMDl0QXBoeDA3clIzNjFmeVFkUmg1b2FlUXhhbHAKVGJzR3NJRnR5WXNDSldCd2NMREJwcGx2eE5TNWxYNGNLNjJ6ZHBFdlJiUVFMaVFESlErWGVDeHUvWDNwb3p0MwpVTldvZHdiVnFEWFgyekYzRUdFc29ZWDZ6L3dLUjQxU2JsWjcyclQvN25xcDlPQXhGaVQwNjNUSUY1WEt3QTByCkw5cisrY0FiOHAwNFhsVmtEU2d4R05MS0JFN3hLV3ZoUEMyeUc0T0d6U1ArN1lDNDlMZC9PaERiWTA0UzlZVUEKbC9mZjh5Qk9KN3dDCi0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0K
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
        crt: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSURCVENDQWUyZ0F3SUJBZ0lJUlpHVUtpci9DRUV3RFFZSktvWklodmNOQVFFTEJRQXdGVEVUTUJFR0ExVUUKQXhNS2EzVmlaWEp1WlhSbGN6QWVGdzB5TlRFd01EZ3hNVFEyTkROYUZ3MHpOVEV3TURZeE1UVXhORE5hTUJVeApFekFSQmdOVkJBTVRDbXQxWW1WeWJtVjBaWE13Z2dFaU1BMEdDU3FHU0liM0RRRUJBUVVBQTRJQkR3QXdnZ0VLCkFvSUJBUUNqSG56dDFsZ2U3WU96dURTeEp1SC8xTVlURzMrdGRjRGU5TFY0bUQvNU5oT1ZMS2t5b25RS0VIUTIKc3N5Nkhyc1RGcnFObFFLZzQzeVhFTHIrTENEVXJNMDhEOStUOGo1MEZ0Z0pSei9aTTFvaG1tVEtBMUYzSlUyeAphRDFKQnlnOFAwSkFqWlhiVFYyYmd2bjlvQVpVOXpzTVQ5a3Q1bFg4YTJvSWF4ZmhPVG9CbXZZNnpZdHpyZHB1Cm9ISHdXZkJIaTArUHN5NHBXaGVndjdZQWllRTJsc1BoMnF5SWRLUVkvcXp5M2JEM2hDWjg4RkZMaStzam5NV0wKRnhYWlg5NS9OWDNHWTZ3QjVQelNycHJ6WnhKbjhjRi9XT3NLWjZDbWloZDJPTDhJb1psMkpmanhQRzlPSUcydgpFbnZYQTBPMExzKzkyR3RwaVo3U21oOThYS0lUQWdNQkFBR2pXVEJYTUE0R0ExVWREd0VCL3dRRUF3SUNwREFQCkJnTlZIUk1CQWY4RUJUQURBUUgvTUIwR0ExVWREZ1FXQkJUNHpaMDA1TVFCaUVDalVjVWZ1cjAwWHVoQWlEQVYKQmdOVkhSRUVEakFNZ2dwcmRXSmxjbTVsZEdWek1BMEdDU3FHU0liM0RRRUJDd1VBQTRJQkFRQm1FVnJLQkVENwpnTWt1VlQ1S2t0czBQRERHcmt4ZWp4M0YyMXE4S2N6QTBRcGpONExueG9qeEI5STVzM2QwZTUwL0N3dW1BMkVWCmJOZEtJZ2x2bG1RMXNGQmtyQnJnRUtoY01IemlGZEVkQUgrMDl0QXBoeDA3clIzNjFmeVFkUmg1b2FlUXhhbHAKVGJzR3NJRnR5WXNDSldCd2NMREJwcGx2eE5TNWxYNGNLNjJ6ZHBFdlJiUVFMaVFESlErWGVDeHUvWDNwb3p0MwpVTldvZHdiVnFEWFgyekYzRUdFc29ZWDZ6L3dLUjQxU2JsWjcyclQvN25xcDlPQXhGaVQwNjNUSUY1WEt3QTByCkw5cisrY0FiOHAwNFhsVmtEU2d4R05MS0JFN3hLV3ZoUEMyeUc0T0d6U1ArN1lDNDlMZC9PaERiWTA0UzlZVUEKbC9mZjh5Qk9KN3dDCi0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0K
    discovery:
        enabled: false # can be disabled
```


port forward and apply config:
```
virtctl port-forward vmi/talos 50000:50000
```
