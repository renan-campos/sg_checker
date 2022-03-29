# Security Group Checker
In AWS, Security Groups are used to control the traffic that instances may receive on specified ports.

### Problem:
Network traffic between instances on a VPC may be blocked if the instance's security groups aren't configured to allow it. We need a way to validate that traffic has been configured to flow between the host network of kubernetes nodes from within the cluster, without querying the underlying AWS infrastructure.

### sg_checker DOES:
- Verify that traffic can flow between nodes
- Can check multiple different ports as specified prior to running.
- Provide simple logs for the end user to understand the current state of the network.

### sg_checker DOES NOT:
- Alter the EC2 instance's security groups

### Here is how it works:
- A checker pod is deployed with the ports that must be checked as command line arguments.
- The checker deploys a scout pod on a different node. 
- The scout runs in the host network namespace and listens for requests from the ports that need checking.
- The checker then sends TCP packets on the needed ports to the scout. 
- It will keep trying until it succeeds.
- When all the ports have been checked, both pods complete.

### To deploy:
1. Create the RBAC permissions needed to run the checker pod:
```kubectl apply -f manifests/rbac.yaml```                                                                                                       


2. Add the ports that need checking to the args field of ```manifests/checker.yaml```:
```
---
apiVersion: v1
kind: Pod
metadata:
   name: net-checker
...
args: [":8080", ":8081"]
...
```


3. Create the checker pod
```kubectl apply -f manifests/checker.yaml```

This approach has been tested locally using minikube, and on an Openshift cluster running in AWS. 
