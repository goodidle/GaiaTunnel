## Gaia Tunnel

run shell scripts

### state
 - 0-prepare
 - 1-SavErr
 - 2-Saved
 - 3-RoleErr
 - 4-StartErr
 - 5-running
 - 6-timeout
 - 7-failed
 - 8-killed
 - 9-success

### example
```
 shell := tunnel.Shell{ Content: "ls", Timeout: 5 }
 shell.Init()
 shell.Start()
```

