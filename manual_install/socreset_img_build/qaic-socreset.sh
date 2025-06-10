#!/bin/bash

########################################################################
# Copyright (c) Qualcomm Technologies, Inc. and/or its subsidiaries.   #
# SPDX-License-Identifier: BSD-3-Clause-Clear.                         #
########################################################################

if [ -f /var/lib/soc_reset_done ]; then
  echo "Soc reset already triggered. Exiting.";
else
  echo "Resetting the Qaic devices..."
  /opt/qti-aic/tools/qaic-util -s && touch /var/lib/qaic_soc_reset_done
fi
echo " Staying alive to keep the pod alive for clean-up."
tail -f /dev/null
