package main

import (
	"fmt"

	appsv1beta1 "k8s.io/api/apps/v1beta1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

// LoadBalancerKind represents a kind of load balancer
type LoadBalancerKind string

const (
	//CRDPlural ...
	CRDPlural string = "shipmentenvironments"

	//CRDGroup ...
	CRDGroup string = "harbor.turner.com"

	//CRDVersion ...
	CRDVersion string = "v1"

	//FullCRDName ...
	FullCRDName string = CRDPlural + "." + CRDGroup

	//LoadBalancerKindElb ...
	LoadBalancerKindElb LoadBalancerKind = "ELB"

	//LoadBalancerKindAlb ...
	LoadBalancerKindAlb LoadBalancerKind = "ALB"
)

//CRD schema

//ShipmentEnvironment represents a CRD
type ShipmentEnvironment struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	Spec              ShipmentEnvironmentSpec   `json:"spec"`
	Status            ShipmentEnvironmentStatus `json:"status,omitempty"`
}

//ShipmentLoadBalancer ...
type ShipmentLoadBalancer struct {
	Kind LoadBalancerKind `json:"kind,omitempty"`
	Type string           `json:"type,omitempty"`
}

//ShipmentEnvironmentSpec represents a ShipmentEnvironment
type ShipmentEnvironmentSpec struct {
	Name         string               `json:"name"`
	Environment  string               `json:"environment"`
	Group        string               `json:"group"`
	Replicas     int32                `json:"replicas"`
	Team         string               `json:"team"`
	Customer     string               `json:"customer"`
	ContactEmail string               `json:"contact-email"`
	LoadBalancer ShipmentLoadBalancer `json:"loadbalancer,omitempty"`
	Containers   []struct {
		Image string `json:"image"`
		Name  string `json:"name"`
		Ports []struct {
			External    bool   `json:"external"`
			Healthcheck string `json:"healthcheck"`
			Name        string `json:"name"`
			Protocol    string `json:"protocol"`
			PublicPort  int32  `json:"public_port"`
			Value       int32  `json:"value"`
		} `json:"ports"`
	} `json:"containers"`
	Envvars []struct {
		Name  string `json:"name"`
		Type  string `json:"type"`
		Value string `json:"value"`
	} `json:"envvars"`
}

//ShipmentEnvironmentStatus represents a ShipmentEnvironmentStatus
type ShipmentEnvironmentStatus struct {
	State   string `json:"state,omitempty"`
	Message string `json:"message,omitempty"`
}

//ShipmentEnvironmentList represents a ShipmentEnvironmentList
type ShipmentEnvironmentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []ShipmentEnvironment `json:"items"`
}

//ShipmentEnvironmentClient represents an object that can talk to the k8s api about our custom resource definition
type ShipmentEnvironmentClient struct {
	cl     *rest.RESTClient
	ns     string
	plural string
}

//Get fetches a ShipmentEnvironment
func (f *ShipmentEnvironmentClient) Get(name string) (*ShipmentEnvironment, error) {
	var result ShipmentEnvironment
	err := f.cl.Get().
		Namespace(f.ns).Resource(f.plural).
		Name(name).Do().Into(&result)
	return &result, err
}

//List lists ShipmentEvironments
func (f *ShipmentEnvironmentClient) List() (*ShipmentEnvironmentList, error) {
	var result ShipmentEnvironmentList
	err := f.cl.Get().
		Namespace(f.ns).Resource(f.plural).
		Do().Into(&result)
	return &result, err
}

//Update lists ShipmentEvironments
func (f *ShipmentEnvironmentClient) Update(obj *ShipmentEnvironment) error {
	var result ShipmentEnvironmentList
	err := f.cl.Put().
		Namespace(f.ns).Resource(f.plural).
		Name(obj.Name).
		Body(obj).Do().Into(&result)
	return err
}

// NewListWatch create a new List watch for our CRD
func (f *ShipmentEnvironmentClient) NewListWatch() *cache.ListWatch {
	return cache.NewListWatchFromClient(f.cl, f.plural, f.ns, fields.Everything())
}

func getShipmentEnvironmentClient(cl *rest.RESTClient, namespace string) *ShipmentEnvironmentClient {
	return &ShipmentEnvironmentClient{
		cl:     cl,
		ns:     namespace,
		plural: CRDPlural,
	}
}

//translates a shipmentenvironment to a k8s deployment and service
func translate(shipmentEnv *ShipmentEnvironment) (*appsv1beta1.Deployment, *apiv1.Service) {

	deployment := appsv1beta1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: shipmentEnv.Name,
		},
		Spec: appsv1beta1.DeploymentSpec{
			Replicas: int32Ptr(shipmentEnv.Spec.Replicas),
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"shipment":    shipmentEnv.Spec.Name,
						"environment": shipmentEnv.Spec.Environment,
					},
				},
				Spec: apiv1.PodSpec{
					Containers: []apiv1.Container{},
				},
			},
		},
	}

	for _, container := range shipmentEnv.Spec.Containers {
		newContainer := apiv1.Container{
			Name:  container.Name,
			Image: container.Image,
			Ports: []apiv1.ContainerPort{},
		}

		for _, port := range container.Ports {
			newContainer.Ports = append(newContainer.Ports, apiv1.ContainerPort{
				Name:          port.Name,
				Protocol:      apiv1.ProtocolTCP,
				ContainerPort: port.Value,
			})
		}

		deployment.Spec.Template.Spec.Containers = append(deployment.Spec.Template.Spec.Containers, newContainer)
	}

	//translate the ShipmentEnvironment into a service
	service := apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: shipmentEnv.Name,
			Labels: map[string]string{
				"shipment":    shipmentEnv.Spec.Name,
				"environment": shipmentEnv.Spec.Environment,
			},
		},
		Spec: apiv1.ServiceSpec{
			Type: apiv1.ServiceTypeNodePort,
			Selector: map[string]string{
				"shipment":    shipmentEnv.Spec.Name,
				"environment": shipmentEnv.Spec.Environment,
			},
			Ports: []apiv1.ServicePort{},
		},
	}

	//does the shipment environment require a load balancer?
	if (ShipmentLoadBalancer{}) != shipmentEnv.Spec.LoadBalancer {
		fmt.Printf("LoadBalancerKind = %v\n", shipmentEnv.Spec.LoadBalancer.Kind)
		if shipmentEnv.Spec.LoadBalancer.Kind == LoadBalancerKindElb {
			service.Spec.Type = apiv1.ServiceTypeLoadBalancer
			fmt.Printf("setting service type = %v\n", service.Spec.Type)
		} else if shipmentEnv.Spec.LoadBalancer.Kind == LoadBalancerKindAlb {
			//todo: create an ingress resource
		}
	}

	//expose all container ports in pod
	for _, container := range shipmentEnv.Spec.Containers {
		for _, port := range container.Ports {
			service.Spec.Ports = append(service.Spec.Ports, apiv1.ServicePort{
				Name:     port.Name,
				Protocol: apiv1.ProtocolTCP,
				Port:     port.PublicPort,
				TargetPort: intstr.IntOrString{
					IntVal: port.Value,
				},
			})
		}
		//TODO: create an ingress resource if port.external == true
	}

	return &deployment, &service
}

func addShipmentEnv(obj interface{}) {
	newShipmentEnv := obj.(*ShipmentEnvironment)
	fmt.Printf("add: %s::%s, status = %v \n", newShipmentEnv.Spec.Name, newShipmentEnv.Spec.Environment, newShipmentEnv.Status.State)

	//TODO: check for existing k8s resources based on this shipment/env (old)

	//only process this add 1 time
	if newShipmentEnv.Status.State == "" {

		//make a deep copy that we can update
		copyObj, err := shipmentEnvironmentScheme.Copy(newShipmentEnv)
		if err != nil {
			fmt.Printf("ERROR creating a deep copy of object: %v\n", err)
			return
		}

		shipmentEnvCopy := copyObj.(*ShipmentEnvironment)
		shipmentEnvCopy.Status = ShipmentEnvironmentStatus{
			State:   "creating",
			Message: "",
		}

		//update the object's status
		err = customClient.Update(shipmentEnvCopy)
		if err != nil {
			fmt.Printf("ERROR updating status: %v\n", err)
		}

		//translate to k8s primitives (deployment, service, ingress, etc)
		deployment, service := translate(shipmentEnvCopy)

		// Create Deployment
		deploymentClient := client.AppsV1beta1().Deployments(apiv1.NamespaceDefault)
		result, err := deploymentClient.Create(deployment)
		if err != nil {
			panic(err)
		}
		fmt.Printf("created deployment %q.\n", result.GetObjectMeta().GetName())

		//create a service
		createdService, err := client.Services(apiv1.NamespaceDefault).Create(service)
		if err != nil {
			panic(err)
		}
		fmt.Printf("created service %q.\n", createdService.GetObjectMeta().GetName())
	}
}

func updateShipmentEnv(oldObj, newObj interface{}) {
	oldShipmentEnv := oldObj.(*ShipmentEnvironment)
	newShipmentEnv := newObj.(*ShipmentEnvironment)

	//ignore status updates
	if oldShipmentEnv.Status.State == "" && newShipmentEnv.Status.State == "creating" ||
		oldShipmentEnv.Status.State == "creating" && newShipmentEnv.Status.State == "running" {
		return
	}

	fmt.Printf("update: \n  old = %s::%s, replicas = %v \n  new = %s::%s, replicas = %v \n",
		oldShipmentEnv.Spec.Name, oldShipmentEnv.Spec.Environment, oldShipmentEnv.Spec.Replicas, newShipmentEnv.Spec.Name, newShipmentEnv.Spec.Environment, newShipmentEnv.Spec.Replicas)

	//update ShipmentEnvironment status based on state of underlying k8s resources
	if newShipmentEnv.Status.State == "creating" {
		deploymentClient := client.AppsV1beta1().Deployments(apiv1.NamespaceDefault)
		result, err := deploymentClient.Get(oldShipmentEnv.Name, metav1.GetOptions{})
		if err != nil {
			panic(fmt.Errorf("Get failed: %+v", err))
		}

		//update status once it's ready
		if result.Status.ReadyReplicas == newShipmentEnv.Spec.Replicas {
			fmt.Println("shipment is ready")

			//make a deep copy that we can update
			copyObj, err := shipmentEnvironmentScheme.Copy(newShipmentEnv)
			if err != nil {
				fmt.Printf("ERROR creating a deep copy of object: %v\n", err)
				return
			}

			shipmentEnvCopy := copyObj.(*ShipmentEnvironment)
			shipmentEnvCopy.Status = ShipmentEnvironmentStatus{
				State:   "running",
				Message: "",
			}

			//update the object's status
			err = customClient.Update(shipmentEnvCopy)
			if err != nil {
				fmt.Printf("ERROR updating status: %v\n", err)
			}
		}
	}
}

func deleteShipmentEnv(obj interface{}) {
	shipmentEnvToDelete := obj.(*ShipmentEnvironment)
	fmt.Printf("delete: %s \n", shipmentEnvToDelete.Name)

	//delete ingress or load balancer first

	deletePolicy := metav1.DeletePropagationForeground
	deleteOptions := metav1.DeleteOptions{
		PropagationPolicy: &deletePolicy,
	}

	//delete service
	fmt.Printf("deleting service: %v \n", shipmentEnvToDelete.Name)
	err := client.Services(apiv1.NamespaceDefault).Delete(shipmentEnvToDelete.Name, &deleteOptions)
	if err != nil {
		panic(err)
	}
	fmt.Println("service deleted")

	//delete deployment
	deploymentClient := client.AppsV1beta1().Deployments(apiv1.NamespaceDefault)
	fmt.Printf("deleting deployment: %v \n", shipmentEnvToDelete.Name)
	err = deploymentClient.Delete(shipmentEnvToDelete.Name, &deleteOptions)
	if err != nil {
		panic(err)
	}
	fmt.Println("deployment deleted")
}
