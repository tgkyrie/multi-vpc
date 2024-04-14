# multi-vpc

在kube-ovn vpc网关上自动创建和维护隧道的operator

## Getting Started

### 环境要求

- go version v1.21.0+
- docker version 17.03+.
- kubectl version v1.11.3+.
- Access to a Kubernetes v1.11.3+ cluster.

### 构建镜像
**执行以下命令， `IMG`为docker仓库名:**

将makefile中的IMG改为镜像地址

```sh
make docker-build docker-push  #到远程仓库
```

### 生成yaml文件

更改了makefile中的deploy

```sh
make deploy
```



## 部署

### k8s集群

```sh
kubectl apply -f deploy.yaml
```

### 测试

新建tunnel.yaml
```yaml
apiVersion: "kubeovn.ustc.io/v1"
kind: VpcNatTunnel
metadata:
  name: ovn-gre0
  namespace: ns2
spec:
  remoteIp: "172.16.50.122" #互联的对端vpc网关实体网络ip
  interfaceAddr: "10.0.0.1/24" #隧道地址
  natGwDp: "vpc2-net1-gateway" #vpc网关名字
  remoteGlobalnetCIDR: "242.0.0.0/16"	#对端集群的globalnet CIDR
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