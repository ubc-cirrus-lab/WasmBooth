Mohammadamin Baqershahi, Changyuan Lin, Visal Saosuo, Paul Chen, and Mohammad Shahrad, "Hierarchical Integration of WebAssembly in Serverless for Efficiency and Interoperability", The 23rd USENIX Symposium on Networked Systems Design and Implementation (NSDI '26).

# Wasabi-WasmBox

WasmBox is a serverless runtime multiplexer based on Webassembly.

WasmBox provides CPU isolation using Linux cgroups and memory isolation using software-based fault isolation (SFI) by Webassembly.

## Setup

### Requirements

To build and run WasmBox, you will need:

- A tool for building container images (e.g., Docker)

- A container runtime or orchestration platform to run WasmBox, such as:

    - Kubernetes (tested with v1.27 – v1.30), or
    - Kubernetes (tested with v1.27 – v1.30) + Knative (tested with v1.15.2), or
    - A standalone container runtime (e.g., containerd, Docker)

- kubectl (required only when deploying to Kubernetes-based platforms)

### Steps (Knative-specific)

1. Build WasmBox:

    ```bash
    docker buildx build  -t <image> .
    ```

2. Configure the required Persistent Volume for Linux cgroups management:
    ```bash
    kubectl apply -f deployment/pv.yaml
    kubectl apply -f deployment/pvc.yaml
    ```

3. Configure a Persistant Volume named `wasmws-wasm-pvc` to store Wasm modules. It could be any filesystem (e.g. Ceph).

4. Configure the image, resource requests and limits, and other parameters for WasmBox in `deployment/wasmbox/ksvc.yaml` or `deployment/wasmbox/ksvc-standalone.yaml`:
    ```bash
    - image: <image_address>
      resources:
          requests:
              cpu: <allocated_cpu>
              memory: <allocated_memory>
          limits:
              cpu: <cpu_limit>
              memory: <memory_limit>
    ```

5. Deploy WasmBox:

    #### Default platform configurations:
    ```bash
    kubectl apply -f deployment/ksvc-standalone.yaml
    ```

    #### With resource-aware Auto-scaling and Queuing configurations:
    ```bash
    kubectl apply -f deployment/ksvc.yaml
    ```

7. Upload and invoke functions
    1. Upload functions by moving the desired Wasm function to the `functions` directory of the configured persistent volume.
    2. Invoke the function with the desired resource limits:
        ```bash
        curl -H 'cpu_quota: <CPU_LIMIT>>' -H 'Memory-Request: <MEMORY_LIMIT>' -v <URL>/<WASM_MODULE_NAME>
        ```

        \* Input data can be send through the HTTP body using POST requests.


## Functions

WasmBox executes user-defined functions compiled to WebAssembly (Wasm). This section outlines general guidelines for writing compatible functions.

### Writing Your Own Functions

Functions must be compiled to WebAssembly using a Wasm toolchain. We recommend using one of the following Wasm compilers/runtimes:

- Wasmtime
    Documentation: https://docs.wasmtime.dev/

- WasmEdge
    Documentation: https://wasmedge.org/docs/

When writing functions, ensure that:

- The function is compiled to a Wasm module compatible with the target runtime.

- Any required inputs/outputs follow the interface expected by WasmBox. (Note: WasmBox expects functions to receive inputs as command-line arguments and produce outputs via standard output (stdout))

- The function does not rely on unsupported system calls or platform-specific features unless explicitly supported by the chosen runtime.

- For improved function execution performance, we recommend using Ahead-of-Time (AOT) compilation rather than Just-in-Time (JIT) compilation, as AOT eliminates runtime compilation overhead and reduces invocation latency.

The exact compilation flags and runtime-specific considerations depend on the compiler/runtime you choose; please refer to the corresponding documentation above for details.
