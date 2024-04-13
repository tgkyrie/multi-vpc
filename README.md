# multi-vpc
### operator 功能
- 在kube-ovn vpc-dns deployment上自动创建和维护Dns转发
- 在kube-ovn vpc网关上自动创建和维护隧道

## Getting Started

### 环境要求
- go version v1.21.0+
- docker version 17.03+.
- kubectl version v1.11.3+.
- Access to a Kubernetes v1.11.3+ cluster.

### 构建镜像
**当前推送到master分支会自动构建镜像，并推送到dockerhub: taowuyuan/multi-vpc:latest**

**你也可以通过以下命令，手动构建和同岁镜像。（`IMG`为构建的镜像名）:**

```sh
export IMG=teriri152/multivpc:latest
make docker-build docker-push
```

### 生成yaml文件

```sh
make deploy
```

### 本地保存镜像文件，远程集群导入镜像文件

本地
```sh
sudo docker tag controller:latest teriri152/multivpc:latest

sudo docker save teriri152/multivpc:latest > myimage.tar
```

k8s集群
```sh
sudo ctr -n k8s.io image import myimage.tar
```

### k8s集群部署
```sh
kubectl apply -f deploy.yaml
```

### 测试 DNS 转发

新建 dns.yaml
```yaml
apiVersion: "kubeovn.ustc.io/v1"
kind: VpcDnsForward
metadata:
  name: dns-1
  namespace: ns1
spec:
  vpc: test-vpc-1

```
```sh
kubectl apply -f dns.yaml
```
登陆vpc-dns deployment, 可以看到路由已经转发
```sh
kubectl delete -f dns.yaml
```
登陆vpc-dns deployment, 可以看到路由已被删除


### 测试 网关隧道
新建 tunnel.yaml

```yaml
apiVersion: "kubeovn.ustc.io/v1"
kind: VpcNatTunnel
metadata:
  name: ovn-gre0
  namespace: ns1
spec:
  remoteIp: "10.10.0.21" #互联的对端vpc网关实体网络ip
  interfaceAddr: "10.100.0.1/24" #隧道地址
  natGwDp: "gw1" #vpc网关名字
  remoteGlobalnetCIDR: "242.0.0.0/16"

```
```sh
kubectl apply -f tunnel.yaml
```
登陆vpc网关pod，可以观察到隧道创建
```sh
kubectl delete -f tunnel.yaml
```
登陆vpc网关pod,可以观察到隧道被删除