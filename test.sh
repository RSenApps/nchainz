case $1 in
  "RESET")
    rm -rf db
    mkdir db
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
esac

