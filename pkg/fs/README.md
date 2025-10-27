# File servers

Interfaces:
- net.http.FileSystem - predates fs.FS interface, there is an adapter.

# SFTP

- chrome ssh extension
- almost all linux machines

https://github.com/pkg/sftp - only "/"

# 9P

- in kernel
- no crypto - great for same machine and 'over secure L4'

From http://9p.cat-v.org/implementations - 4 golang impl
- https://github.com/docker-archive/go-p9p  - archived 2020
- https://code.google.com/archive/p/go9p/source/default/source - seems obsolete, 2015
- https://github.com/Harvey-OS/ninep - 2019 - but claims to be stable

Not listed:
- https://github.com/droyo/styx - 2 year since last push

Not listed but appears active:
- https://github.com/knusbaum/go9p
  - https://github.com/knusbaum/go9p/blob/master/cmd/mount9p/main.go - FUSE
  - https://github.com/knusbaum/go9p/blob/master/cmd/import9p/main.go - uses kubectl to run export9p
  - export9p can use stdin/stdout, uds, tcp


## Servers


## Client

- v9fs - native kernel

# APIs

- os.File, etc - used by sshfs
- 

# FUSE 

Virtiofs also work using serialized FUSE over virtqueue.`

## GCP Fuse driver

GKE: addons GcsFuseCsiDriver to auto-inject, enabled on autopilot and cloudrun.



gcloud storage buckets add-iam-policy-binding gs://${BUCKET_NAME} \
    --member "principal://iam.googleapis.com/projects/${PROJECT_NUMBER}/locations/global/workloadIdentityPools/${PROJECT_ID}.svc.id.goog/subject/ns/${NAMESPACE}/sa/${KSA_NAME}" \
    --role "roles/storage.objectUser"

For all buckets in the project:
gcloud projects add-iam-policy-binding ${GCS_PROJECT} ...

Or 
--member serviceAccount:${GKE_PROJECT_ID}.svc.id.goog[dns-system/default]
or group:.../allAuthenticatedUsers/

On GKE, annotations are used - and it creates a sidecar container (gke-gcsfuse/[cpu-limit|memory-limit|ephemeral-storage-limit|cpu-request|memory-request|ephemeral-storage-request])
Default 250 m CPU, 256M, 5 G ephemeral storage.

Container name: gke-gcsfuse-sidecar ( can be configured explicitly )

Cloudrun:
```yaml
spec:
      containers:
      - image: IMAGE_URL
        volumeMounts:
        - name: VOLUME_NAME
          mountPath: MOUNT_PATH
      volumes:
      - name: VOLUME_NAME
        csi:
          driver: gcsfuse.run.googleapis.com
          #readOnly: IS_READ_ONLY
          volumeAttributes:
            bucketName: BUCKET_NAME

Also supports:
nfs:
  server: IP_ADDRESS
  path: NFS_PATH
  readOnly: IS_READ_ONLY
  
        emptyDir:
          sizeLimit: SIZE_LIMIT
          medium: Memory
```

Cloudrun also support mount -t 9p -o trans=tcp,aname=/mnt/diod,version=9p2000.L,uname=root,access=user IP_ADDRESS MOUNT_POINT_DIRECTORY
nbd, cifs

gcsfuse GLOBAL_OPTIONS BUCKET_NAME MOUNT_POINT

code: go install github.com/googlecloudplatform/gcsfuse/v2@master
Deps:
- otel, contrib/otelgrpc, controb/grpc-gateway, 
