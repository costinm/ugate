# Syncthing support

Syncthing is a modern decentralised file/directory sync server.

At it's core, it maintains a database for each sync root, and a protocol to send/receive changes.
It runs as a server using inotify or periodic scans - so change propagation can be very fast if inotify is set properly.

Besides the nice UI and well documented uses, there are few interesting concepts and more interesting uses.

# Concerns

Syncthing invents its own directory and relay systems, as well as a custom protocol.

While the core rsync+database is great - it may work better on top of SSH or H2/H3, with
standard mTLS. 

Also, a proper NFS-style with caching may be more effective in a lot of cases - syncthing is still
an option for workloads with no priviledges to mount anything.

## Proper use

For multi-master sync - not friendly to databases or similar large files with multiple changes in the middle.
That includes things like home directory - the chrome databases and a lot of other things would fail.

It does work well enough for master-slave, with one computer doing writes and the rest have read-only copies.
Still not ideal for replicating databases ( and structured files), better to use native protocols.

Great for source files, doc dirs (with few concurrent writes), config files.


## Identity

- mTLS based
- Device ID is SHA256 of certificate

## Public or private infrastructure

Syncthing can work in an isolated mode - using local discovery for node IPs - however this doesn't work
very well in VPCs if broadcast is not supported. 


Like Tor, bittorrent or IPFS - it is possible to operate it independent of the public infra, or to use the public infra.


## Protocols

1. Discovery - Global Discovery v3 
   - JSON with []addresses
   - tcp://:22000
   - Local v4 - broadcast UDP on port 21027, MC on FF12:8384, 30 to 60 sec interval, retransmit when new discovered
   - Magic, ID, []addresses, instance_id - to detect restart
2. Relay - 
   - bep-relay TLS ALPN
   - Join relay, wait for messages from relay to connect (SessionInvitation with key), sends JoinSessionRequest
   - Connect and request a session with a waiting server. 
   - Both end do mTLS - but there is no certificate signed by common authority. 
3. Filesystem/Blob storage with sync - block exchange protocol v1


## Others

- rsync - no database, no realtime. Good for daily backups.
  - librsync - used in duplicity and other tools
  - rdiff and rdiff-backup - use rsync for backup
  - duplicity - can save to S3, sftp, etc
  - zsync - for downloads, iso images - over HTTP with .zsync files for the rolling hashes.
  - rclone
