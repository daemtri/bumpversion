package bumpver

import (
	"reflect"
	"testing"
)

var deployment = []byte(`kind: Deployment
apiVersion: apps/v1
metadata:
  name: box-core
spec:
  replicas: 1
  selector:
    matchLabels:
      app: box-core
  template:
    metadata:
      labels:
        app: box-core
    spec:
      volumes:
        - name: configs
          configMap:
            name: box-core
            defaultMode: 420
      containers:
        - name: container-box-core
          image: "harbor-ali.imeete.com/box/box-core:1.1.15"
          ports:
            - name: http-80
              containerPort: 80
              protocol: TCP
          resources: {}
          volumeMounts:
            - name: configs
              readOnly: true
              mountPath: /app/.env
              subPath: .env
          imagePullPolicy: Always
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxUnavailable: 25%
      maxSurge: 25%
  revisionHistoryLimit: 10
  progressDeadlineSeconds: 600`)

var deploymentResult = []byte(`kind: Deployment
apiVersion: apps/v1
metadata:
  name: box-core
spec:
  replicas: 1
  selector:
    matchLabels:
      app: box-core
  template:
    metadata:
      labels:
        app: box-core
    spec:
      volumes:
        - name: configs
          configMap:
            name: box-core
            defaultMode: 420
      containers:
        - name: container-box-core
          image: "harbor-ali.imeete.com/box/box-core:1.1.16"
          ports:
            - name: http-80
              containerPort: 80
              protocol: TCP
          resources: {}
          volumeMounts:
            - name: configs
              readOnly: true
              mountPath: /app/.env
              subPath: .env
          imagePullPolicy: Always
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxUnavailable: 25%
      maxSurge: 25%
  revisionHistoryLimit: 10
  progressDeadlineSeconds: 600`)

func TestBumpYamlImageVersion(t *testing.T) {
	type args struct {
		yamlBytes []byte
		image     string
		tag       string
	}
	tests := []struct {
		name    string
		args    args
		want    []byte
		wantErr bool
	}{
		{
			name: "test-taml",
			args: args{
				yamlBytes: deployment,
				image:     "harbor-ali.imeete.com/box/box-core",
				tag:       "1.1.16",
			},
			want:    deploymentResult,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BumpYamlImageVersion(tt.args.yamlBytes, tt.args.image, tt.args.tag)
			if (err != nil) != tt.wantErr {
				t.Errorf("BumpYamlImageVersion() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("BumpYamlImageVersion() = %v, want %v", got, tt.want)
			}
		})
	}
}
