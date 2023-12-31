/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	webappv1 "my.domain/subprocess/api/v1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// SubprocessReconciler reconciles a Subprocess object
type SubprocessReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=webapp.my.domain,resources=subprocesses,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=webapp.my.domain,resources=subprocesses/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=webapp.my.domain,resources=subprocesses/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Subprocess object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.15.0/pkg/reconcile
func (r *SubprocessReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	// TODO(user): your logic here
	myFinalizerName := "batch.tutorial.kubebuilder.io/finalizer"
	deleted := false
	var sp = &webappv1.Subprocess{}
	err := r.Client.Get(ctx, req.NamespacedName, sp)

	if err != nil {
		deleted = true
		fmt.Printf("Failed to get subprocess %s in request: %s", req.NamespacedName, err)
	}
	fmt.Printf("Subprocess content: %v\n", sp)
	if deleted || !sp.ObjectMeta.DeletionTimestamp.IsZero() {
		fmt.Printf("Subprocess is being deleted\n")
		labelSelector := labels.Set{
			"crd": req.Name,
		}.AsSelector()
		listOpts := []client.ListOption{
			// Namespace: default, Modify this according to your needs
			client.InNamespace("default"),
			client.MatchingLabelsSelector{Selector: labelSelector},
		}
		deployments := &appsv1.DeploymentList{}
		if err := r.Client.List(context.Background(), deployments, listOpts...); err != nil {
			panic(err.Error())
		}
		fmt.Printf("Found %d deployments\n", len(deployments.Items))
		if len(deployments.Items) != 0 {
			if err := r.Client.Delete(context.Background(), &deployments.Items[0]); err != nil {
				//result, err := deploymentsClient.Create(context.TODO(), deploy, metav1.CreateOptions{})

				panic(err)
			}
			fmt.Printf("Deployment %s deleted successfully!\n", deployments.Items[0].Name)
		}
		configMaps := &corev1.ConfigMapList{}
		if err := r.Client.List(context.Background(), configMaps, listOpts...); err != nil {
			panic(err.Error())
		}
		fmt.Printf("Found %d configmaps\n", len(configMaps.Items))
		if len(configMaps.Items) != 0 {
			if err := r.Client.Delete(context.Background(), &configMaps.Items[0]); err != nil {
				//result, err := deploymentsClient.Create(context.TODO(), deploy, metav1.CreateOptions{})
				panic(err)
			}
			fmt.Printf("ConfigMap %s deleted successfully!\n", configMaps.Items[0].Name)
		}

		if controllerutil.ContainsFinalizer(sp, myFinalizerName) {
			// remove our finalizer from the list and update it.
			controllerutil.RemoveFinalizer(sp, myFinalizerName)
			if err := r.Update(ctx, sp); err != nil {
				return ctrl.Result{}, err
			}
		}
	} else {
		conf := `
[supervisorctl]
[supervisord]
nodaemon=true
logfile=/var/log/supervisord.log
`
		for i := 0; i < len(sp.Spec.Commands); i++ {
			conf += fmt.Sprintf("[program: command_%d]\n%s\n", i, sp.Spec.Commands[i])
		}
		fmt.Printf("ConfigMap content: %s\n", conf)
		configMap := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-configmap",
				Namespace: "default",
				Labels: map[string]string{
					"crd": sp.Name,
				},
			},
			Data: map[string]string{
				"supervisord.conf": conf,
			},
		}
		if err := r.Client.Create(ctx, configMap); err != nil {
			return ctrl.Result{}, err
		}

		fmt.Println("ConfigMap created successfully!")

		// Create a new Deployment
		deploy := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-deployment",
				Namespace: "default",
				Labels: map[string]string{
					"crd": sp.Name,
				},
			},

			Spec: appsv1.DeploymentSpec{
				Replicas: int32Ptr(2), // Modify this to specify the number of replicas
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": "my-app",
					},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app": "my-app",
						},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:            "my-app",
								Image:           "goproject:test",
								ImagePullPolicy: "Never",
								VolumeMounts: []corev1.VolumeMount{
									{
										Name:      "my-configmap",
										MountPath: "/etc/supervisor/conf.d",
										ReadOnly:  true,
									},
								},
							},
						},
						Volumes: []corev1.Volume{
							{
								Name: "my-configmap",
								VolumeSource: corev1.VolumeSource{
									ConfigMap: &corev1.ConfigMapVolumeSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: "my-configmap",
										},
									},
								},
							},
						},
					},
				},
			},
		}
		fn := func() error {
			return nil
		}
		// Create Deployment
		if _, err := controllerutil.CreateOrUpdate(context.Background(), r.Client, deploy, fn); err != nil {
			//result, err := deploymentsClient.Create(context.TODO(), deploy, metav1.CreateOptions{})

			panic(err)
		}
		fmt.Printf("Created deployment %s.\n", deploy.GetObjectMeta().GetName())

		// if !controllerutil.ContainsFinalizer(deploy, myFinalizerName) {
		// 	controllerutil.AddFinalizer(deploy, myFinalizerName)
		// 	if err := r.Update(ctx, deploy); err != nil {
		// 		return ctrl.Result{}, err
		// 	}
		// }
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SubprocessReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&webappv1.Subprocess{}).
		Complete(r)
}
func int32Ptr(i int32) *int32 { return &i }
