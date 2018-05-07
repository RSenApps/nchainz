case $1 in
  "RESET")
    rm -rf db
    mkdir db
    go install
    ;;
  "STARTNODES")    
    go install      
    dex node localhost:4000 & dex node localhost:4001 & dex node localhost:4002
    ;;
  "STOPNODES")    
    pkill -f "dex"
    ;;
  "TEST1")
    dex transfer 500 NATIVE Satoshi x
    for i in `seq 1 10`; do
      (dex transfer 400 NATIVE x Satoshi) &
    done
    ;;
  "TEST2")
    for i in `seq 1 50`; do
      (dex transfer 100 NATIVE Satoshi x)
    done
    dex claim 1000 NATIVE Satoshi 
    ;; 
  *)
    echo "Command not found: try again"
    ;;
esac

