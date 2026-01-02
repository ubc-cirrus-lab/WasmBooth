# Deploy a Production Ready Kubernetes Cluster

## Kubespray
Kubespray Relase v2.24.1 (Kubernetes v1.29.2)
```
git clone https://github.com/kubernetes-sigs/kubespray.git
git checkout 2cb8c85 
```

## Components
* containerd v1.7.13
* Calico v3.27.2
* MetalLB v0.13.9 (Layer 2 Mode)
* Rancher Local Path Provisioner v0.0.24
* Metrics Server v0.7.0

## Configurations
* kube_proxy_mode: iptables
* containerd_storage_dir: /mnt/lib/containerd
* etcd_data_dir: /var/lib/etcd
* local_path_provisioner_claim_root: /mnt/local-path-provisioner/
