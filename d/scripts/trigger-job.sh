#!/bin/bash

# Default values
KERNEL_SIZE=15
INPUT_PATH="/data/input"
OUTPUT_PATH="/data/output"
INPUT_FILE=""

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -k|--kernel)
            KERNEL_SIZE="$2"
            shift 2
            ;;
        -i|--input)
            INPUT_PATH="$2"
            shift 2
            ;;
        -o|--output)
            OUTPUT_PATH="$2"
            shift 2
            ;;
        -f|--file)
            INPUT_FILE="$2"
            shift 2
            ;;
        -h|--help)
            echo "Usage: $0 [options]"
            echo "Options:"
            echo "  -k, --kernel SIZE     Gaussian kernel size (default: 15)"
            echo "  -i, --input PATH      Input directory path (default: /data/input)"
            echo "  -o, --output PATH     Output directory path (default: /data/output)"
            echo "  -f, --file FILENAME   Specific file to process (optional)"
            echo "  -h, --help           Show this help message"
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
done

# Create temporary job manifest
TEMP_JOB="/tmp/image-processor-job-$$.yaml"

# Replace placeholders in template
sed -e "s|{{KERNEL_SIZE}}|$KERNEL_SIZE|g" \
    -e "s|{{INPUT_PATH}}|$INPUT_PATH|g" \
    -e "s|{{OUTPUT_PATH}}|$OUTPUT_PATH|g" \
    -e "s|{{INPUT_FILE}}|$INPUT_FILE|g" \
    k8s/job-template.yaml > "$TEMP_JOB"

# Create the job
echo "Creating image processor job with:"
echo "  Kernel size: $KERNEL_SIZE"
echo "  Input path: $INPUT_PATH"
echo "  Output path: $OUTPUT_PATH"
if [ -n "$INPUT_FILE" ]; then
    echo "  Input file: $INPUT_FILE"
fi

kubectl create -f "$TEMP_JOB"

# Clean up
rm -f "$TEMP_JOB"

# Get job name
JOB_NAME=$(kubectl get jobs -l app=image-processor --sort-by=.metadata.creationTimestamp -o jsonpath='{.items[-1].metadata.name}')

echo ""
echo "Job created: $JOB_NAME"
echo ""
echo "To check status:"
echo "  kubectl get job $JOB_NAME"
echo ""
echo "To view logs:"
echo "  kubectl logs -l job-name=$JOB_NAME"
echo ""
echo "To delete job:"
echo "  kubectl delete job $JOB_NAME"