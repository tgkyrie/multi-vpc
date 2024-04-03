package controller

import (
	testv1 "VpcConnection/api/v1"
	"context"
	"fmt"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
)

type GatewayController struct {
	Client client.Client
	Config *rest.Config
}

func New(client client.Client, config *rest.Config) *GatewayController {
	return &GatewayController{Client: client, Config: config}
}

func (r *GatewayController) Start(ctx context.Context) error {
	clientSet, err := kubernetes.NewForConfig(r.Config)
	var vpcConnectionList testv1.VpcConnectionList
	if err != nil {
		return err
	}
	labelSelector := labels.Set{
		"ovn.kubernetes.io/vpc-nat-gw": "true",
	}
	informer := cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options v1.ListOptions) (runtime.Object, error) {
				options.LabelSelector = labelSelector.AsSelector().String()
				return clientSet.AppsV1().StatefulSets("kube-system").List(ctx, options)
			},
			WatchFunc: func(options v1.ListOptions) (watch.Interface, error) {
				options.LabelSelector = labelSelector.AsSelector().String()
				return clientSet.AppsV1().StatefulSets("kube-system").Watch(ctx, options)
			},
		},
		&appsv1.StatefulSet{},
		0,
		cache.Indexers{},
	)
	_, err = informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			statefulSet := obj.(*appsv1.StatefulSet)
			gatewayName := strings.TrimPrefix(statefulSet.Name, "vpc-nat-gw-")
			labelsSet := map[string]string{
				"gateway": gatewayName,
			}
			option := client.ListOptions{
				LabelSelector: labels.SelectorFromSet(labelsSet),
			}
			err = r.Client.List(ctx, &vpcConnectionList, &option)
			if err != nil {
				return
			}
			if statefulSet.Status.AvailableReplicas == 1 {
				for _, vpcConnection := range vpcConnectionList.Items {
					vpcConnection.Spec.Operation = testv1.VpcConnectionRecovery
					err = r.Client.Update(ctx, &vpcConnection)
					if err != nil {
						return
					}
				}
			}
		},
		UpdateFunc: func(old, new interface{}) {
			oldStatefulSet := old.(*appsv1.StatefulSet)
			newStatefulSet := new.(*appsv1.StatefulSet)
			gatewayName := strings.TrimPrefix(newStatefulSet.Name, "vpc-nat-gw-")
			labelsSet := map[string]string{
				"gateway": gatewayName,
			}
			option := client.ListOptions{
				LabelSelector: labels.SelectorFromSet(labelsSet),
			}
			err = r.Client.List(ctx, &vpcConnectionList, &option)
			if err != nil {
				return
			}
			if oldStatefulSet.Status.AvailableReplicas == 0 && newStatefulSet.Status.AvailableReplicas == 1 {
				for _, vpcConnection := range vpcConnectionList.Items {
					vpcConnection.Spec.Operation = testv1.VpcConnectionRecovery
					err = r.Client.Update(ctx, &vpcConnection)
					if err != nil {
						return
					}
				}
			}
			if oldStatefulSet.Status.AvailableReplicas == 1 && newStatefulSet.Status.AvailableReplicas == 0 {
				for _, vpcConnection := range vpcConnectionList.Items {
					vpcConnection.Spec.Operation = testv1.VpcConnectionStop
					err = r.Client.Update(ctx, &vpcConnection)
					if err != nil {
						return
					}
				}
			}
		},
		DeleteFunc: func(obj interface{}) {
			statefulSet := obj.(*appsv1.StatefulSet)
			klog.Info(statefulSet)
			gatewayName := strings.TrimPrefix(statefulSet.Name, "vpc-nat-gw-")
			labelsSet := map[string]string{
				"gateway": gatewayName,
			}
			option := client.ListOptions{
				LabelSelector: labels.SelectorFromSet(labelsSet),
			}
			err = r.Client.List(ctx, &vpcConnectionList, &option)
			if err != nil {
				return
			}
			for _, vpcConnection := range vpcConnectionList.Items {
				vpcConnection.Spec.Operation = testv1.VpcConnectionStop
				err = r.Client.Update(ctx, &vpcConnection)
				if err != nil {
					return
				}
			}
		},
	})
	if err != nil {
		return err
	}
	stopCh := make(chan struct{})
	defer close(stopCh)
	go informer.Run(stopCh)
	if !cache.WaitForCacheSync(stopCh, informer.HasSynced) {
		return fmt.Errorf("Error syncing cache\n")
	}
	select {
	case <-stopCh:
		klog.Info("Received termination signal, exiting")
	}
	return nil
}
