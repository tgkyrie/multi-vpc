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
**执行以下命令， `IMG`为docker仓库名:**

**你也可以通过以下命令，手动构建和同岁镜像。（`IMG`为构建的镜像名）:**

```sh
make docker-build docker-push  #到远程仓库
```

### 生成yaml文件

```sh
make deploy
```



## 部署

### k8s集群

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
  remoteIp: "172.16.50.121" #互联的对端vpc网关实体网络ip
  interfaceAddr: "10.0.0.1/24" #隧道地址
  natGwDp: "vpc2-net1-gateway" #vpc网关名字，不要带"vpc-nat-gw-"
  type: "vxlan" #隧道类型，或"gre"
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



## TODO
+ Watch Vpc网关Pod的变更事件，维护隧道
+ ...