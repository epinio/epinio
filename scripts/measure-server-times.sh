#/bin/bash

trial(){
    echo 1>&2 "$@"
    ep debug load "$@"
}

sep() {
    echo '________ ________ ______ ________ | ______     ________ | __________  __________ | __________ __________ __________ __________'
}

#     count millis streams
#
trial 100   1000 1
trial 100   500  1
trial 100   200  1
trial 100   100  1
trial 100   50   1
trial 100   20   1
trial 100   10   1
trial 100   5    1
trial 100   2    1
trial 100   1    1
sep
#
trial 100   1000 1
trial 100   1000 2
trial 100   1000 5
trial 100   1000 10
trial 100   1000 20
trial 100   1000 50
trial 100   1000 100
trial 100   1000 200
trial 100   1000 500
trial 100   1000 1000
sep
#
trial 100   100  1
trial 100   100  2
trial 100   100  5
trial 100   100  10
trial 100   100  20
trial 100   100  50
trial 100   100  100
trial 100   100  200
trial 100   100  500
trial 100   100  1000
