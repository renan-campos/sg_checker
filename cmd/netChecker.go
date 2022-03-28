package main

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"os"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func main() {

	localNodeName, found := os.LookupEnv("NODE_NAME")
	if !found {
		fmt.Fprintf(os.Stderr,
			"NODE_NAME environment variable not found\n")
		os.Exit(1)
	}
	localImageName, found := os.LookupEnv("IMAGE_NAME")
	if !found {
		fmt.Fprintf(os.Stderr,
			"IMAGE_NAME environment variabled not found\n")
		os.Exit(1)
	}
	localNamespace, found := os.LookupEnv("NAMESPACE")
	if !found {
		fmt.Fprintf(os.Stderr,
			"NAMESPACE environment variabled not found\n")
		os.Exit(1)
	}
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr,
			"Expected a list of ports ':<port>' as command line arguments\n")
		os.Exit(1)
	}
	ports := os.Args[1:]

	fmt.Println("Setting up k8s client")
	k8sClient, err := setupK8sClient()
	if err != nil {
		panic(err.Error())
	}

	fmt.Println("Finding remote node")
	var remoteNode struct {
		Name    string
		Address string
	}
	nodeList, err := k8sClient.CoreV1().Nodes().List(
		context.Background(), metav1.ListOptions{
			LabelSelector: "node-role.kubernetes.io/worker"})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get nodes: %s", err)
		os.Exit(1)
	}

	fmt.Println("listing nodes")
	for _, node := range nodeList.Items {
		fmt.Println(node.Name)
		if node.Name == localNodeName {
			continue
		}
		remoteNode.Name = node.Name
		for _, addr := range node.Status.Addresses {
			if addr.Type == corev1.NodeInternalIP {
				remoteNode.Address = addr.Address
				break
			}
		}
		break
	}
	if remoteNode.Name == "" {
		fmt.Fprintf(os.Stderr, "Failed to find a remote node\n")
		os.Exit(1)
	}

	fmt.Printf("Creating scout job on node %s with address %s\n",
		remoteNode.Name, remoteNode.Address)

	scoutCommand := []string{"netScout"}
	for _, port := range ports {
		// TODO: port validation
		scoutCommand = append(scoutCommand, port)
	}

	scoutJobRequest := batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: "net-scout",
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name: "net-scout",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:    "net-scout",
							Image:   localImageName,
							Command: scoutCommand,
						},
					},
					NodeName:      remoteNode.Name,
					HostNetwork:   true,
					RestartPolicy: corev1.RestartPolicyNever,
				},
			},
		},
	}
	err = createJob(k8sClient, &scoutJobRequest, localNamespace)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create job: %s\n", err)
		os.Exit(1)
	}

	fmt.Println("Checking access to ports")
	for _, port := range ports {
		scoutEndpoint := fmt.Sprintf("%s%s", remoteNode.Address, port)
		fmt.Printf("Checking access to %s\n", scoutEndpoint)
		for {
			conn, err := net.Dial("tcp", scoutEndpoint)
			if err != nil {
				fmt.Fprintf(os.Stderr,
					"error occurred connecting to %s: %s\n\tRetrying...\n",
					scoutEndpoint, err)
				time.Sleep(time.Second)
				continue
			}
			_, err = fmt.Fprintf(conn, "ping")
			if err != nil {
				fmt.Fprintf(os.Stderr,
					"failed to send packet to %s: %s\n\tRetrying...\n",
					scoutEndpoint, err)
				time.Sleep(time.Second)
				continue
			}

			var msg []byte = make([]byte, 5)
			_, err = bufio.NewReader(conn).Read(msg)
			if err != nil {
				fmt.Fprintf(os.Stderr,
					"failed to receive packet from %s: %s\n\tIgnoring...\n",
					scoutEndpoint, err)
			}
			fmt.Printf("Message received: %s\n", string(msg))
			break
		}
	}

	fmt.Println("Validation successfull! Deleting scout job")
	err = k8sClient.BatchV1().Jobs(localNamespace).Delete(
		context.Background(), scoutJobRequest.Name, metav1.DeleteOptions{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to delete scout job: %s", err)
	}

}

func setupK8sClient() (*kubernetes.Clientset, error) {
	// To run locally:
	// Point the environment variable LOCAL_RUN to your kubeconfig
	localKubeconfig, found := os.LookupEnv("LOCAL_RUN")
	if found {
		config, err := clientcmd.BuildConfigFromFlags("", localKubeconfig)
		if err != nil {
			panic(err.Error())
		}

		return kubernetes.NewForConfig(config)
	}

	// creates the in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	// creates the clientset
	return kubernetes.NewForConfig(config)
}

func createJob(k8sClient *kubernetes.Clientset, job *batchv1.Job, namespace string) error {
	_, err := k8sClient.BatchV1().Jobs(namespace).Create(
		context.Background(), job, metav1.CreateOptions{})
	if err != nil {
		if k8serrors.IsAlreadyExists(err) {
			err = k8sClient.BatchV1().Jobs(namespace).Delete(
				context.Background(), job.Name, metav1.DeleteOptions{})
		}
		for {
			_, err = k8sClient.BatchV1().Jobs(namespace).Get(
				context.Background(), job.Name, metav1.GetOptions{})
			if err != nil {
				if k8serrors.IsNotFound(err) {
					break
				}
				return err
			}
			time.Sleep(time.Microsecond * 25)
		}
		_, err = k8sClient.BatchV1().Jobs(namespace).Create(
			context.Background(), job, metav1.CreateOptions{})
	}
	return err
}
