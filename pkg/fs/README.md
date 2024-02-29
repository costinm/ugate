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
