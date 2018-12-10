# distributed-mutual-exclusion
This repository contains several distributed mutual exclusion algorithms implementation which includes `Centralized Mutual Server`, `Ricart Agrawala`, `Lamport Distributed Mutual Exclusion` and `Token Ring`. It's being noted all of them cannot meet practical requirements of modern distributed system, hence, they are just conducive to the learning of principles of distributed mutual exclusion algorithm.  Specifically,  we assume they can only provide following guarantees:
- Safety
- Fairness
- Low message overhead
- No bottlenecks
- Tolerate out-of-order messages

In other word, all of them suffers from following limitations:
-  Allow processes to join protocol or to drop out
-  Tolerate failed processes
-  Tolerate dropped messages

## About Test
 `TCP` (reliable) protocol is adopted to implement communication layer for all of four algorithms (`go` language). The test process consists of two separate phases:
 
 **Phase a**. Each process repeats the following procedures several times independently.

1. Perform local operations, which sleep for an interval between [100, 300]ms.
2. Start entering the critical section. Execute the acquisition mutex code.
3. Execute the critical section . Accumulating a global shared variable, which randomly increases the it by a number between [1,10] every 100ms during the [100, 200] ms period. At the mean time, record these accumulated intermediate values by a global array and saving to file.
4. Exit the critical section. Execute the release mutex code.

**Phase b**. Each process repeats the following procedures several times independently.

1. For a specific process, if the number of its PID is even, then sleep for an interval between  [100, 300]ms first, and then repeat the `Phase a` operation process. Processes with odd PID repeat the `Phase b` process directly.

The verification of the test results of the four algorithms focuses on two aspects:

- **Algorithm correctness**. Ensure mutual access to shared resources by checking records of global arrays in `Phase a&b`. In addition, verify the access log file for the process access shared resources.
- **Bandwidth and latency**. Count the number of messages being read and written for each process, and obtain the delay of the mutex algorithm, and calculate the average delay.

