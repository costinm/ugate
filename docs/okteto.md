# Okteto notes

Okteto is a CLI that modifies a deployment, replacing the 
container with a custom image running "syncthing" and "sshd'

It will run a local server, maintaining port forwards and a 
local "syncthing", so local directory will be in sync with the 
remote directory.

This allows local development with an IDE, and running 
commands in the K8S environment.

## SSH

On local host, it creates:

```shell
Host MANIFEST_NAME.okteto
  HostName localhost
  Port PORT
  StrictHostKeyChecking no
  UserKnownHostsFile /dev/null
```



## Config 

In container, it uses 'go' and 'gocache' volumes.




## Installing

```shell
  curl https://get.okteto.com -sSfL | sh
  
  # Will provide a shell terminal in the swapped container.
  okteto up
```

## Commands

```shell

okteto exec ...

```

## Internals

- init container copies files from a 'dev' image to user container
- creates a persistent volume (reduce churn)
- creates a secret with configs
- SSH HAS A HARDCODED HOST KEY !!!, and custom SSH server


Generated deployment example:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  annotations:
    deployment.kubernetes.io/revision: "1"
    dev.okteto.com/auto-create: up
    dev.okteto.com/deployment: '{"metadata":{"name":"dev-istiod","namespace":"istio-system","creationTimestamp":null,"annotations":{"dev.okteto.com/auto-create":"up"}},"spec":{"replicas":1,"selector":{"matchLabels":{"app":"dev-istiod"}},"template":{"metadata":{"creationTimestamp":null,"labels":{"app":"dev-istiod"}},"spec":{"containers":[{"name":"dev","image":"gcr.io/istio-testing/build-tools:master-2021-05-07T04-05-01","resources":{},"imagePullPolicy":"Always"}],"terminationGracePeriodSeconds":0}},"strategy":{"type":"Recreate"}},"status":{}}'
    dev.okteto.com/revision: "1"
    dev.okteto.com/stignore: 600188e9d3997a42167a7914dc1e191c
    dev.okteto.com/version: "1.0"
  labels:
    dev.okteto.com: "true"
  name: dev-istiod
  namespace: istio-system
spec:
  progressDeadlineSeconds: 600
  replicas: 1
  revisionHistoryLimit: 10
  selector:
    matchLabels:
      app: dev-istiod
  strategy:
    type: Recreate
  template:
    metadata:
      annotations:
        dev.okteto.com/stignore: 600188e9d3997a42167a7914dc1e191c
      labels:
        app: dev-istiod
        dev.okteto.com: "true"
        interactive.dev.okteto.com: dev-istiod
    spec:
      containers:
      - args:
        - -r
        - -s
        - authorized_keys:/var/okteto/remote/authorized_keys
        - -s
        - .stignore:/work/.stignore
        command:
        - /var/okteto/bin/start.sh
        env:
        - name: TAG
          value: "16"
        - name: HUB
          value: costinm
        - name: BUILD_WITH_CONTAINER
          value: "0"
        - name: HOME
          value: /home/istio-proxy
        - name: USER
          value: istio-proxy
        - name: WEBHOOK
          value: istiod
        - name: ISTOD_ADDR
          value: istiod.istio-system.svc:15012
        - name: OKTETO_NAMESPACE
          value: istio-system
        - name: OKTETO_NAME
          value: dev-istiod
        image: gcr.io/istio-testing/build-tools:master-2021-05-07T04-05-01
        imagePullPolicy: Always
        name: dev
        resources:
          requests:
            cpu: "8"
            memory: 16G
        securityContext:
          capabilities:
            add:
            - SYS_PTRACE
            - NET_ADMIN
          runAsGroup: 1337
          runAsUser: 1337
        terminationMessagePath: /dev/termination-log
        terminationMessagePolicy: File
        volumeMounts:
        - mountPath: /var/syncthing
          name: okteto-dev-istiod
          subPath: syncthing
        - mountPath: /var/okteto/remote
          name: okteto-dev-istiod
          subPath: okteto-remote
        - mountPath: /go/pkg/
          name: okteto-dev-istiod
          subPath: data/go/pkg
        - mountPath: /work
          name: okteto-dev-istiod
          subPath: src
        - mountPath: /var/syncthing/secret/
          name: okteto-sync-secret
        - mountPath: /var/okteto/secret/
          name: okteto-dev-secret
        - mountPath: /var/okteto/bin
          name: okteto-bin
        workingDir: /work
      dnsPolicy: ClusterFirst
      initContainers:
      - command:
        - sh
        - -c
        - cp /usr/local/bin/* /okteto/bin
        image: okteto/bin:1.2.29
        imagePullPolicy: IfNotPresent
        name: okteto-bin
        resources: {}
        terminationMessagePath: /dev/termination-log
        terminationMessagePolicy: File
        volumeMounts:
        - mountPath: /okteto/bin
          name: okteto-bin
      restartPolicy: Always
      schedulerName: default-scheduler
      securityContext:
        fsGroup: 3000
      terminationGracePeriodSeconds: 0
      volumes:
      - name: okteto-sync-secret
        secret:
          defaultMode: 420
          items:
          - key: config.xml
            mode: 292
            path: config.xml
          - key: cert.pem
            mode: 292
            path: cert.pem
          - key: key.pem
            mode: 292
            path: key.pem
          secretName: okteto-dev-istiod
      - name: okteto-dev-istiod
        persistentVolumeClaim:
          claimName: okteto-dev-istiod
      - name: okteto-dev-secret
        secret:
          defaultMode: 420
          items:
          - key: dev-secret-authorized_keys
            mode: 420
            path: authorized_keys
          - key: dev-secret-.stignore
            mode: 420
            path: .stignore
          secretName: okteto-dev-istiod
      - emptyDir: {}
        name: okteto-bin
```
