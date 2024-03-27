# multi-vpc
在kube-ovn vpc网关上自动创建和维护隧道的operator

## Getting Started

### 环境要求
- go version v1.21.0+
- docker version 17.03+.
- kubectl version v1.11.3+.
- Access to a Kubernetes v1.11.3+ cluster.

### 构建镜像
**执行以下命令， `IMG`为构建的镜像名:**

```sh
make docker-build # 保存到镜像controller
make docker-build # 保存到镜像controller
```


### 生成yaml文件

```sh
# cp config/crd/bases/kubeovn.ustc.io_vpcnattunnels.yaml ./deploy.yaml
bin/kustomize-v5.3.0 build config/default > deploy.yaml
```

### 本地保存镜像文件，远程集群导入镜像文件

本地
```sh
# sudo docker tag controller:latest multivpc:latest

sudo docker save -o myimage.tar controller:latest
#sudo docker save github.com/tgkyrie/k8splay/tunnel-controller/multivpc:latest > myimage.tar

```

k8s集群
```sh
sudo ctr -n k8s.io image import myimage.tar
```

### k8s集群部署
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
  internalIp: "172.16.50.84" #vpc网关ip
  remoteIp: "172.16.50.122" #互联的对端vpc网关ip
  interfaceAddr: "10.0.0.1/24" #隧道地址
  remoteInterfaceAddr: "10.0.0.2/24" #对端隧道地址
  natGwDp: "vpc2-net1-gateway" #vpc网关名字

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
+ 处理VpcNatTunnel Update事件
+ Watch Vpc网关Pod的变更事件，维护隧道
+ ...