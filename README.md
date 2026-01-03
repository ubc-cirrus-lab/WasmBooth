# Wasabi-WasmBox

WasmBox is a serverless runtime multiplexer based on Webassembly.

WasmBox provides CPU isolation using Linux cgroups and memory isolation using software-based fault isolation (SFI) by Webassembly.

## Setup

### Requirements

- A tool to build WasmBox into a container image (e.g. Docker)
- A container orchestration platform (i.e. Knative, Kubernetes, etc) or a just a container runtime (e.g. containerd, Docker, etc) to run WasmBox.
- kubectl (when deploying on Kubernetes-based platforms)

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
