#!/bin/bash

########################################################################
# Copyright (c) Qualcomm Technologies, Inc. and/or its subsidiaries.   #
# SPDX-License-Identifier: BSD-3-Clause-Clear.                         #
########################################################################

set -o pipefail

# Configuration
readonly SOC_RESET_MARKER="/var/lib/qaic_soc_reset_done"
readonly FIRMWARE_DIR="/opt/qti-aic/firmware"
readonly FW_CONFIG_FILE="/var/lib/firmware/fw2_swe.json"
readonly QMONITOR_PORT="${QMONITOR_PORT:-62472}"
readonly DEVICE_BOOT_WAIT="${DEVICE_BOOT_WAIT:-3}"
readonly MAX_RETRIES="${MAX_RETRIES:-5}"
readonly RETRY_DELAY=2


# Track QMonitor PID
QMONITOR_PID=""

# Logging function with timestamps
log() {
    echo "[$(date +'%Y-%m-%d %H:%M:%S')] $*"
}

log_error() {
    echo "[$(date +'%Y-%m-%d %H:%M:%S')] ERROR: $*" >&2
}

# Check if devices are ready
wait_for_devices() {
    local retries=0
    log "Waiting for AIC devices to be ready..."
    sleep "$DEVICE_BOOT_WAIT"

    while [ $retries -lt "$MAX_RETRIES" ]; do
        # Check for accel devices in sysfs
        if [ -d /sys/class/accel ]; then
            local device_count
            device_count=$(ls -1d /sys/class/accel/accel* 2>/dev/null | wc -l)
            if [ "$device_count" -gt 0 ]; then
                log "Found $device_count AIC device(s) in sysfs"
                log "AIC devices are ready"
                return 0
            fi
        fi
        retries=$((retries + 1))
        log "Waiting for devices... (attempt $retries/$MAX_RETRIES)"
        sleep "$RETRY_DELAY"
    done

    log_error "AIC devices did not become ready after $MAX_RETRIES attempts"
    return 1
}

# Perform SOC reset
perform_soc_reset() {
    if [ -f "$SOC_RESET_MARKER" ]; then
        log "SOC reset already completed (marker file exists). Skipping reset."
        return 0
    fi

    log "Resetting the QAIC devices..."
    if /opt/qti-aic/tools/qaic-util -s; then
        if touch "$SOC_RESET_MARKER"; then
            log "SOC reset completed successfully"
            return 0
        else
            log_error "Failed to create marker file: $SOC_RESET_MARKER"
            return 1
        fi
    else
        log_error "qaic-util -s command failed"
        return 1
    fi
}

# Prepare QMonitor configuration
# Copies the firmware configuration file needed by QMonitor gRPC server
prepare_qmonitor_config() {
    log "Preparing QMonitor configuration..."

    if ! mkdir -p "$FIRMWARE_DIR"; then
        log_error "Failed to create firmware directory: $FIRMWARE_DIR"
        return 1
    fi

    if [ -f "$FW_CONFIG_FILE" ]; then
        log "Copying QMonitor configuration: $FW_CONFIG_FILE -> $FIRMWARE_DIR/"
        if cp "$FW_CONFIG_FILE" "$FIRMWARE_DIR/"; then
            log "QMonitor configuration copied successfully"
            return 0
        else
            log_error "Failed to copy QMonitor configuration"
            return 1
        fi
    else
        log "QMonitor configuration not found: $FW_CONFIG_FILE"
        log "QMonitor cannot be started without this file"
        return 1
    fi
}

# Start QMonitor server
start_qmonitor() {
    log "Starting QMonitor gRPC server on port $QMONITOR_PORT to enable SBL updates"

    # Run QMonitor in background and track PID
    /opt/qti-aic/tools/qaic-monitor-grpc-server -v &
    QMONITOR_PID=$!

    log "QMonitor started with PID: $QMONITOR_PID"

    # Wait for QMonitor to exit
    wait $QMONITOR_PID
    local exit_code=$?
    log_error "QMonitor exited unexpectedly with code: $exit_code"
    return $exit_code
}

# Cleanup handler
cleanup() {
    log "Received termination signal, cleaning up..."

    # Kill QMonitor gracefully if running
    if [ -n "$QMONITOR_PID" ] && kill -0 "$QMONITOR_PID" 2>/dev/null; then
        log "Stopping QMonitor (PID: $QMONITOR_PID)..."
        kill -TERM "$QMONITOR_PID" 2>/dev/null

        # Wait up to 10 seconds for graceful shutdown
        local wait_count=0
        while kill -0 "$QMONITOR_PID" 2>/dev/null && [ $wait_count -lt 3 ]; do
            sleep 1
            wait_count=$((wait_count + 1))
        done

        # Force kill if still running
        if kill -0 "$QMONITOR_PID" 2>/dev/null; then
            log "QMonitor didn't stop gracefully, forcing termination..."
            kill -KILL "$QMONITOR_PID" 2>/dev/null
        else
            log "QMonitor stopped gracefully"
        fi
    fi

    exit 0
}

# Main execution
main() {
    log "=== QAIC SOC Reset Script Started ==="

    # Set up signal handlers
    trap cleanup SIGTERM SIGINT

    # Perform SOC reset
    if ! perform_soc_reset; then
        log_error "SOC reset failed, but continuing..."
    fi

    # Prepare QMonitor configuration (can be done while devices boot)
    if ! prepare_qmonitor_config; then
        log_error "QMonitor configuration preparation failed"
        log "Keeping container alive for debugging..."
        exec tail -f /dev/null
    fi

    # Wait for devices to boot and become ready
    log "Waiting ${DEVICE_BOOT_WAIT}s for AIC devices to boot after reset..."
    sleep "$DEVICE_BOOT_WAIT"

    if ! wait_for_devices; then
        log_error "Devices not ready after reset"
        log "QMonitor requires ready devices, cannot start"
        log "Keeping container alive for debugging..."
        exec tail -f /dev/null
    fi

    # Start QMonitor (requires both config file and ready devices)
    start_qmonitor

    # If QMonitor exits, keep container alive for debugging
    log "QMonitor has stopped, keeping container alive for debugging"
    exec tail -f /dev/null
}

# Run main function
main