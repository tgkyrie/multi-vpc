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

以下即为部署成功：

```sh
sdn@server02:~$ kubectl get pod -A
multi-vpc-system      multi-vpc-controller-manager-7cf6c6b9d6-q4sq5    2/2     Running            0               73s
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
sdn@server10:~$ kubectl describe deployment vpc-dns-test-dns1 -n kube-system
Pod Template:
  Labels:           k8s-app=vpc-dns-test-dns1
  Annotations:      k8s.v1.cni.cncf.io/networks: default/ovn-nad
                    ovn-nad.default.ovn.kubernetes.io/logical_switch: ovn-default
                    ovn.kubernetes.io/logical_switch: vpc2-net1
  Service Account:  vpc-dns
  Init Containers:
   init-route:
    Image:      docker.io/kubeovn/vpc-nat-gateway:v1.12.8
    Port:       <none>
    Host Port:  <none>
    Command:
      sh
      -c
      ip -4 route add 10.96.0.1 via 10.244.0.1 dev net1;ip -4 route add 218.2.2.2 via 10.244.0.1 dev net1;ip -4 route add 114.114.114.114 via 10.244.0.1 dev net1;ip -4 route add 10.96.0.10 via 10.244.0.1 dev net1;
```



```sh
kubectl delete -f dns.yaml
```
登陆vpc-dns deployment, 可以看到路由已被删除

```sh
sdn@server10:~$ kubectl describe deployment vpc-dns-test-dns1 -n kube-system
Pod Template:
  Labels:           k8s-app=vpc-dns-test-dns1
  Annotations:      k8s.v1.cni.cncf.io/networks: default/ovn-nad
                    ovn-nad.default.ovn.kubernetes.io/logical_switch: ovn-default
                    ovn.kubernetes.io/logical_switch: vpc2-net1
  Service Account:  vpc-dns
  Init Containers:
   init-route:
    Image:      docker.io/kubeovn/vpc-nat-gateway:v1.12.8
    Port:       <none>
    Host Port:  <none>
    Command:
      sh
      -c
      ip -4 route add 10.96.0.1 via 10.244.0.1 dev net1;ip -4 route add 218.2.2.2 via 10.244.0.1 dev net1;ip -4 route add 114.114.114.114 via 10.244.0.1 dev net1;
```




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
sdn@server10:~$ kubectl exec -it -n kube-system vpc-nat-gw-vpc2-net1-gateway-0 -- /bin/sh
/kube-ovn # ifconfig
ovn-gre0  Link encap:Ethernet  HWaddr 7E:9F:E9:59:81:01
          inet addr:10.0.0.1  Bcast:0.0.0.0  Mask:255.255.255.0
          inet6 addr: fe80::7c9f:e9ff:fe59:8101/64 Scope:Link
          UP BROADCAST RUNNING MULTICAST  MTU:1450  Metric:1
          RX packets:0 errors:0 dropped:0 overruns:0 frame:0
          TX packets:9 errors:0 dropped:0 overruns:0 carrier:0
          collisions:0 txqueuelen:1000
          RX bytes:0 (0.0 B)  TX bytes:600 (600.0 B)
/kube-ovn # ip route
10.0.0.0/24 dev ovn-gre0 proto kernel scope link src 10.0.0.1
242.0.0.0/16 via 10.0.1.1 dev eth0
242.1.0.0/16 dev ovn-gre0 scope link
/kube-ovn # iptables -t nat -L POSTROUTING
SNAT       all  --  anywhere             242.1.0.0/16         to:242.0.0.1-242.0.0.8
```



```sh
kubectl delete -f tunnel.yaml
```
登陆vpc网关pod,可以观察到以上内容均被删除



## TODO
+ Watch Vpc网关Pod的重启事件，维护隧道
+ ...



## Directory Structure

* api: crd的定义
* bin: bin kubebuilder的一些编译工具
* cmd: main函数起始点nfig
* config: kubuilder生成的配置文件，包括CRD 定义文件、RBAC 角色配置文件等
* internal: 包括crd状态变更时的代码逻辑
* sample: crd的例子
* test: kubebuilder自带的测试，由于我们都是在集群上进行测试，因此未使用