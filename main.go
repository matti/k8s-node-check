package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var clientset *kubernetes.Clientset

func getPods() (*corev1.PodList, error) {
	return clientset.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{
		LabelSelector: "app=k8s-node-check",
	})
}

func deletePod(pod corev1.Pod) {
	clientset.CoreV1().Pods(pod.Namespace).Delete(context.TODO(), pod.Name, metav1.DeleteOptions{})
}

func deletePodForce(pod corev1.Pod) {
	force := int64(0)

	clientset.CoreV1().Pods(pod.Namespace).Delete(context.TODO(), pod.Name, metav1.DeleteOptions{
		GracePeriodSeconds: &force,
	})
}

func updateNodeStatus(node *corev1.Node) {
	var nodeConditions []corev1.NodeCondition

	for _, nodeCondition := range node.Status.Conditions {
		if nodeCondition.Type == corev1.NodePIDPressure {
			nodeCondition.LastHeartbeatTime = metav1.Now()
			nodeCondition.LastTransitionTime = metav1.Now()
			nodeCondition.Message = "k8s-node-check"
			nodeCondition.Status = corev1.ConditionTrue
		}

		nodeConditions = append(nodeConditions, nodeCondition)
	}
	node.Status.Conditions = nodeConditions

	clientset.CoreV1().Nodes().UpdateStatus(context.TODO(), node, metav1.UpdateOptions{})
}
func main() {
	var settings struct {
		create    time.Duration
		terminate time.Duration
		every     time.Duration
	}

	flag.DurationVar(&settings.create, "create", 10*time.Second, "")
	flag.DurationVar(&settings.terminate, "terminate", 15*time.Second, "")
	flag.DurationVar(&settings.every, "every", 5*time.Second, "")

	flag.Parse()
	var config *rest.Config
	var err error

	if path, ok := os.LookupEnv("KUBECONFIG"); !ok {
		if config, err = rest.InClusterConfig(); err != nil {
			panic(err)
		}
	} else {
		if config, err = clientcmd.BuildConfigFromFlags("", path); err != nil {
			panic(err)
		}
	}

	if clientset, err = kubernetes.NewForConfig(config); err != nil {
		panic(err)
	}

	for {
		pods, err := getPods()
		if err != nil {
			time.Sleep(time.Second * 1)
			continue
		}

		for _, pod := range pods.Items {
			deletePod(pod)
		}
		break
	}

	for {
		startedAt := time.Now()

		nodes, err := clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			log.Println("node list error", err)
			continue
		}

		nodeNames := make(map[string]*corev1.Node)
		for _, node := range nodes.Items {
			nodeNames[node.Name] = node.DeepCopy()
		}

		if pods, err := clientset.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{}); err != nil {
			continue
		} else {
			for _, pod := range pods.Items {
				if pod.ObjectMeta.DeletionTimestamp == nil {
					continue
				}
				node := nodeNames[pod.Spec.NodeName]
				if node == nil {
					continue
				}

				nodeAge := time.Since(node.CreationTimestamp.Time)
				podAge := time.Since(pod.CreationTimestamp.Time)

				if podAge > nodeAge {
					deletePodForce(pod)
				}
			}
		}

		for _, node := range nodes.Items {
			if node.Spec.Unschedulable {
				//log.Println("node", node.Name, "unschedulable")
				continue
			}

			ready := false
			for _, condition := range node.Status.Conditions {
				if condition.Type == "Ready" && condition.Status == "True" {
					ready = true
					break
				}
			}

			if !ready {
				//log.Println("node", node.Name, "not ready")
				continue
			}

			podSpec := &corev1.Pod{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Pod",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: fmt.Sprintf("k8s-node-check-%s", node.ObjectMeta.GetUID()),
					Labels: map[string]string{
						"app": "k8s-node-check",
					},
				},
				Spec: corev1.PodSpec{
					NodeName: node.Name,
					Containers: []corev1.Container{
						{
							Name:  "bause",
							Image: "ghcr.io/matti/bause:user",
						},
					},
				},
			}

			if _, err := clientset.CoreV1().Pods("default").Create(context.TODO(), podSpec, metav1.CreateOptions{}); err != nil {
				if !strings.Contains(err.Error(), "already exists") {
					log.Println("pod create failed", err)
				}
			}
		}

		pods, err := getPods()
		if err != nil {
			continue
		}

		for _, pod := range pods.Items {
			node := nodeNames[pod.Spec.NodeName]
			if node == nil {
				//log.Println("node", pod.Spec.NodeName, "no longer present, deleting pod", pod.Name)
				deletePod(pod)
				continue
			}

			podAge := time.Since(pod.GetCreationTimestamp().Time)
			nodeAge := time.Since(node.GetObjectMeta().GetCreationTimestamp().Time)

			switch pod.Status.Phase {
			case "Pending":
				if nodeAge < time.Minute*1 {
					//log.Println("node", node.Name, nodeAge, "too young")
					break
				}

				if podAge > settings.create {
					log.Println("PROBLEM", "CREATE", pod.Spec.NodeName, podAge)
					updateNodeStatus(node)
				}
			case "Running":
				if pod.ObjectMeta.DeletionTimestamp == nil {
					// Running
					deletePod(pod)
				} else if pod.ObjectMeta.DeletionGracePeriodSeconds != nil {
					// Terminating
					deletionGracePeriodSecondsDuration := time.Duration(*pod.ObjectMeta.DeletionGracePeriodSeconds) * time.Second
					inTerminating := deletionGracePeriodSecondsDuration - time.Until(pod.ObjectMeta.DeletionTimestamp.Time)

					if inTerminating > settings.terminate {
						log.Println("PROBLEM", "TERMINATING", pod.Spec.NodeName, inTerminating)
						updateNodeStatus(node)
					}
				}
			default:
				log.Println("UNKNOWN", "PHASE", pod.Status.Phase)
			}
		}

		took := time.Since(startedAt)
		remaining := settings.every - took
		if remaining > 0 {
			time.Sleep(remaining)
		}
	}
}
