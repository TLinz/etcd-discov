# etcd-discov

A simple services discovery example built on etcd.

1. Run servers on different ports
    
    ```go run ./server/main.go -Port 3000```

    ```go run ./server/main.go -Port 3001```

    ```go run ./server/main.go -Port 3002```

2. Run client
   
    ```go run ./client/main.go```