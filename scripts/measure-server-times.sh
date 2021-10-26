#/bin/bash

trial(){
    echo >2 "$@"
    ep debug load "$@"
}

#     count millis

trial 100   1000
trial 100   500
trial 100   200
trial 100   100
trial 100   50
trial 100   20
trial 100   10
trial 100   5
trial 100   2
trial 100   1

trial 10    10
trial 20    10
trial 30    10
trial 40    10
trial 50    10
trial 60    10
trial 70    10
trial 80    10
trial 90    10

trial 1000  1000
trial 1000  500
trial 1000  200
trial 1000  100
trial 1000  50
trial 1000  20
trial 1000  10
trial 1000  5
trial 1000  2
trial 1000  1
