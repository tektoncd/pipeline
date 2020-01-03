package workspace_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/workspace"
	"github.com/tektoncd/pipeline/test/names"
	corev1 "k8s.io/api/core/v1"
)

func TestGetVolumes(t *testing.T) {
	names.TestingSeed()
	for _, tc := range []struct {
		name            string
		workspaces      []v1alpha1.WorkspaceBinding
		expectedVolumes map[string]corev1.Volume
	}{{
		name: "binding a single workspace with a PVC",
		workspaces: []v1alpha1.WorkspaceBinding{{
			Name: "custom",
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: "mypvc",
			},
			SubPath: "/foo/bar/baz",
		}},
		expectedVolumes: map[string]corev1.Volume{
			"custom": {
				Name: "ws-9l9zj",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: "mypvc",
					},
				},
			},
		},
	}, {
		name: "binding a single workspace with emptyDir",
		workspaces: []v1alpha1.WorkspaceBinding{{
			Name: "custom",
			EmptyDir: &corev1.EmptyDirVolumeSource{
				Medium: corev1.StorageMediumMemory,
			},
			SubPath: "/foo/bar/baz",
		}},
		expectedVolumes: map[string]corev1.Volume{
			"custom": {
				Name: "ws-mz4c7",
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{
						Medium: corev1.StorageMediumMemory,
					},
				},
			},
		},
	}, {
		name: "binding a single workspace with configMap",
		workspaces: []v1alpha1.WorkspaceBinding{{
			Name: "custom",
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: "foobarconfigmap",
				},
				Items: []corev1.KeyToPath{{
					Key:  "foobar",
					Path: "foobar.txt",
				}},
			},
			SubPath: "/foo/bar/baz",
		}},
		expectedVolumes: map[string]corev1.Volume{
			"custom": {
				Name: "ws-mssqb",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "foobarconfigmap",
						},
						Items: []corev1.KeyToPath{{
							Key:  "foobar",
							Path: "foobar.txt",
						}},
					},
				},
			},
		},
	}, {
		name: "binding a single workspace with secret",
		workspaces: []v1alpha1.WorkspaceBinding{{
			Name: "custom",
			Secret: &corev1.SecretVolumeSource{
				SecretName: "foobarsecret",
				Items: []corev1.KeyToPath{{
					Key:  "foobar",
					Path: "foobar.txt",
				}},
			},
			SubPath: "/foo/bar/baz",
		}},
		expectedVolumes: map[string]corev1.Volume{
			"custom": {
				Name: "ws-78c5n",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: "foobarsecret",
						Items: []corev1.KeyToPath{{
							Key:  "foobar",
							Path: "foobar.txt",
						}},
					},
				},
			},
		},
	}, {
		name:            "0 workspace bindings",
		workspaces:      []v1alpha1.WorkspaceBinding{},
		expectedVolumes: map[string]corev1.Volume{},
	}, {
		name: "binding multiple workspaces",
		workspaces: []v1alpha1.WorkspaceBinding{{
			Name: "custom",
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: "mypvc",
			},
			SubPath: "/foo/bar/baz",
		}, {
			Name: "even-more-custom",
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: "myotherpvc",
			},
		}},
		expectedVolumes: map[string]corev1.Volume{
			"custom": {
				Name: "ws-6nl7g",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: "mypvc",
					},
				},
			},
			"even-more-custom": {
				Name: "ws-j2tds",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: "myotherpvc",
					},
				},
			},
		},
	}, {
		name: "multiple workspaces binding to the same volume with diff subpaths doesnt duplicate",
		workspaces: []v1alpha1.WorkspaceBinding{{
			Name: "custom",
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: "mypvc",
			},
			SubPath: "/foo/bar/baz",
		}, {
			Name: "custom2",
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: "mypvc",
			},
			SubPath: "/very/professional/work/space",
		}},
		expectedVolumes: map[string]corev1.Volume{
			"custom": {
				Name: "ws-vr6ds",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: "mypvc",
					},
				},
			},
			"custom2": {
				// Since it is the same PVC source, it can't be added twice with two different names
				Name: "ws-vr6ds",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: "mypvc",
					},
				},
			},
		},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			v := workspace.GetVolumes(tc.workspaces)
			if d := cmp.Diff(tc.expectedVolumes, v); d != "" {
				t.Errorf("Didn't get expected volumes from bindings (-want, +got): %s", d)
			}
		})
	}
}

func TestApply(t *testing.T) {
	names.TestingSeed()
	for _, tc := range []struct {
		name             string
		ts               v1alpha1.TaskSpec
		workspaces       []v1alpha1.WorkspaceBinding
		expectedTaskSpec v1alpha1.TaskSpec
	}{{
		name: "binding a single workspace with a PVC",
		ts: v1alpha1.TaskSpec{
			Workspaces: []v1alpha1.WorkspaceDeclaration{{
				Name: "custom",
			}},
		},
		workspaces: []v1alpha1.WorkspaceBinding{{
			Name: "custom",
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: "mypvc",
			},
			SubPath: "/foo/bar/baz",
		}},
		expectedTaskSpec: v1alpha1.TaskSpec{
			StepTemplate: &corev1.Container{
				VolumeMounts: []corev1.VolumeMount{{
					Name:      "ws-9l9zj",
					MountPath: "/workspace/custom",
					SubPath:   "/foo/bar/baz",
				}},
			},
			Volumes: []corev1.Volume{{
				Name: "ws-9l9zj",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: "mypvc",
					},
				},
			}},
			Workspaces: []v1alpha1.WorkspaceDeclaration{{
				Name: "custom",
			}},
		},
	}, {
		name: "binding a single workspace with emptyDir",
		ts: v1alpha1.TaskSpec{
			Workspaces: []v1alpha1.WorkspaceDeclaration{{
				Name: "custom",
			}},
		},
		workspaces: []v1alpha1.WorkspaceBinding{{
			Name: "custom",
			EmptyDir: &corev1.EmptyDirVolumeSource{
				Medium: corev1.StorageMediumMemory,
			},
			SubPath: "/foo/bar/baz",
		}},
		expectedTaskSpec: v1alpha1.TaskSpec{
			StepTemplate: &corev1.Container{
				VolumeMounts: []corev1.VolumeMount{{
					Name:      "ws-mz4c7",
					MountPath: "/workspace/custom",
					SubPath:   "/foo/bar/baz", // TODO: what happens when you use subPath with emptyDir
				}},
			},
			Volumes: []corev1.Volume{{
				Name: "ws-mz4c7",
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{
						Medium: corev1.StorageMediumMemory,
					},
				},
			}},
			Workspaces: []v1alpha1.WorkspaceDeclaration{{
				Name: "custom",
			}},
		},
	}, {
		name: "task spec already has volumes and stepTemplate",
		ts: v1alpha1.TaskSpec{
			StepTemplate: &corev1.Container{
				VolumeMounts: []corev1.VolumeMount{{
					Name:      "awesome-volume",
					MountPath: "/",
				}},
			},
			Volumes: []corev1.Volume{{
				Name: "awesome-volume",
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			}},
			Workspaces: []v1alpha1.WorkspaceDeclaration{{
				Name: "custom",
			}},
		},
		workspaces: []v1alpha1.WorkspaceBinding{{
			Name: "custom",
			EmptyDir: &corev1.EmptyDirVolumeSource{
				Medium: corev1.StorageMediumMemory,
			},
			SubPath: "/foo/bar/baz",
		}},
		expectedTaskSpec: v1alpha1.TaskSpec{
			StepTemplate: &corev1.Container{
				VolumeMounts: []corev1.VolumeMount{{
					Name:      "awesome-volume",
					MountPath: "/",
				}, {
					Name:      "ws-mssqb",
					MountPath: "/workspace/custom",
					SubPath:   "/foo/bar/baz", // TODO: what happens when you use subPath with emptyDir
				}},
			},
			Volumes: []corev1.Volume{{
				Name: "awesome-volume",
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			}, {
				Name: "ws-mssqb",
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{
						Medium: corev1.StorageMediumMemory,
					},
				},
			}},
			Workspaces: []v1alpha1.WorkspaceDeclaration{{
				Name: "custom",
			}},
		},
	}, {
		name: "0 workspace bindings",
		ts: v1alpha1.TaskSpec{
			Steps: []v1alpha1.Step{{
				Container: corev1.Container{
					Name: "foo",
				}}},
		},
		workspaces: []v1alpha1.WorkspaceBinding{},
		expectedTaskSpec: v1alpha1.TaskSpec{
			Steps: []v1alpha1.Step{{
				Container: corev1.Container{
					Name: "foo",
				}}},
		},
	}, {
		name: "binding multiple workspaces",
		ts: v1alpha1.TaskSpec{
			Workspaces: []v1alpha1.WorkspaceDeclaration{{
				Name: "custom",
			}, {
				Name: "even-more-custom",
			}},
		},
		workspaces: []v1alpha1.WorkspaceBinding{{
			Name: "custom",
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: "mypvc",
			},
			SubPath: "/foo/bar/baz",
		}, {
			Name: "even-more-custom",
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: "myotherpvc",
			},
		}},
		expectedTaskSpec: v1alpha1.TaskSpec{
			StepTemplate: &corev1.Container{
				VolumeMounts: []corev1.VolumeMount{{
					Name:      "ws-78c5n",
					MountPath: "/workspace/custom",
					SubPath:   "/foo/bar/baz",
				}, {
					Name:      "ws-6nl7g",
					MountPath: "/workspace/even-more-custom",
					SubPath:   "",
				}},
			},
			Volumes: []corev1.Volume{{
				Name: "ws-78c5n",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: "mypvc",
					},
				},
			}, {
				Name: "ws-6nl7g",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: "myotherpvc",
					},
				},
			}},
			Workspaces: []v1alpha1.WorkspaceDeclaration{{
				Name: "custom"}, {
				Name: "even-more-custom",
			}},
		},
	}, {
		name: "multiple workspaces binding to the same volume with diff subpaths doesnt duplicate",
		ts: v1alpha1.TaskSpec{
			Workspaces: []v1alpha1.WorkspaceDeclaration{{
				Name: "custom",
			}, {
				Name: "custom2",
			}},
		},
		workspaces: []v1alpha1.WorkspaceBinding{{
			Name: "custom",
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: "mypvc",
			},
			SubPath: "/foo/bar/baz",
		}, {
			Name: "custom2",
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: "mypvc",
			},
			SubPath: "/very/professional/work/space",
		}},
		expectedTaskSpec: v1alpha1.TaskSpec{
			StepTemplate: &corev1.Container{
				VolumeMounts: []corev1.VolumeMount{{
					Name:      "ws-j2tds",
					MountPath: "/workspace/custom",
					SubPath:   "/foo/bar/baz",
				}, {
					Name:      "ws-j2tds",
					MountPath: "/workspace/custom2",
					SubPath:   "/very/professional/work/space",
				}},
			},
			Volumes: []corev1.Volume{{
				Name: "ws-j2tds",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: "mypvc",
					},
				},
			}},
			Workspaces: []v1alpha1.WorkspaceDeclaration{{
				Name: "custom",
			}, {
				Name: "custom2",
			}},
		},
	}, {
		name: "non default mount path",
		ts: v1alpha1.TaskSpec{
			Workspaces: []v1alpha1.WorkspaceDeclaration{{
				Name:      "custom",
				MountPath: "/my/fancy/mount/path",
			}},
		},
		workspaces: []v1alpha1.WorkspaceBinding{{
			Name: "custom",
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: "mypvc",
			},
		}},
		expectedTaskSpec: v1alpha1.TaskSpec{
			StepTemplate: &corev1.Container{
				VolumeMounts: []corev1.VolumeMount{{
					Name:      "ws-l22wn",
					MountPath: "/my/fancy/mount/path",
				}},
			},
			Volumes: []corev1.Volume{{
				Name: "ws-l22wn",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: "mypvc",
					},
				},
			}},
			Workspaces: []v1alpha1.WorkspaceDeclaration{{
				Name:      "custom",
				MountPath: "/my/fancy/mount/path",
			}},
		},
	}, {
		name: "readOnly true marks volume mount readOnly",
		ts: v1alpha1.TaskSpec{
			Workspaces: []v1alpha1.WorkspaceDeclaration{{
				Name:      "custom",
				MountPath: "/my/fancy/mount/path",
				ReadOnly:  true,
			}},
		},
		workspaces: []v1alpha1.WorkspaceBinding{{
			Name: "custom",
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: "mypvc",
			},
		}},
		expectedTaskSpec: v1alpha1.TaskSpec{
			StepTemplate: &corev1.Container{
				VolumeMounts: []corev1.VolumeMount{{
					Name:      "ws-twkr2",
					MountPath: "/my/fancy/mount/path",
					ReadOnly:  true,
				}},
			},
			Volumes: []corev1.Volume{{
				Name: "ws-twkr2",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: "mypvc",
					},
				},
			}},
			Workspaces: []v1alpha1.WorkspaceDeclaration{{
				Name:      "custom",
				MountPath: "/my/fancy/mount/path",
				ReadOnly:  true,
			}},
		},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			ts, err := workspace.Apply(tc.ts, tc.workspaces)
			if err != nil {
				t.Fatalf("Did not expect error but got %v", err)
			}
			if d := cmp.Diff(tc.expectedTaskSpec, *ts); d != "" {
				t.Errorf("Didn't get expected TaskSpec modifications (-want, +got): %s", d)
			}
		})
	}
}
