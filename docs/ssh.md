# Using SSH with HBONE

The dev image includes sshd and a script to start it.

```shell
 # Get latest version of the ugate CLI for tunnel
 go install github.com/costinm/ugate/cmd/hbone@latest

 # Address of the deployment
 GATE=ugatevm-yydsuf6tpq-uc.a.run.app:443

 ssh -v  -o StrictHostKeyChecking=no \
   -o ProxyCommand='hbone https://$GATE/dm/127.0.0.1:22' root@$GATE
```
