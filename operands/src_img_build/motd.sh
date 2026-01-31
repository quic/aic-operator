#!/bin/sh

################################################################################
# Copyright (c) Qualcomm Technologies, Inc. and/or its subsidiaries. All rights reserved.
# Confidential and Proprietary - Qualcomm Technologies, Inc. and/or its subsidiaries.
################################################################################

if [ -r /opt/qti-aic/versions/platform.xml ]; then
    platform_version=$(
        cat /opt/qti-aic/versions/platform.xml |
            grep -o '<base_version>.*</base_version>' |
            sed -n 's/<.*>\(.*\)<.*>/\1/p'
    )
fi

if [ -r /opt/qti-aic/versions/platform.xml ]; then
    platform_build_id=$(
        cat /opt/qti-aic/versions/platform.xml |
            grep -o '<build_id>.*</build_id>' |
            sed -n 's/<.*>\(.*\)<.*>/\1/p'
    )
fi

if [ -r /opt/qti-aic/versions/apps.xml ]; then
    apps_version=$(
        cat /opt/qti-aic/versions/apps.xml |
            grep -o '<base_version>.*</base_version>' |
            sed -n 's/<.*>\(.*\)<.*>/\1/p'
    )
fi

if [ -r /opt/qti-aic/versions/apps.xml ]; then
    apps_build_id=$(
        cat /opt/qti-aic/versions/apps.xml |
            grep -o '<build_id>.*</build_id>' |
            sed -n 's/<.*>\(.*\)<.*>/\1/p'
    )
fi

platform_sdk_ver_str=""
apps_sdk_ver_str=""

if [ -n "${platform_version}" ]; then
    platform_sdk_ver_str="Platform SDK version: ${platform_version}.${platform_build_id}"
else
    platform_sdk_ver_str="Platform SDK not installed"
fi

if [ -n "${apps_version}" ]; then
    apps_sdk_ver_str="Apps SDK version: ${apps_version}.${apps_build_id}"
else
    apps_sdk_ver_str="Apps SDK not installed"
fi

cat <<EOM
==================================
== Qualcomm Cloud AI Containers ==
==================================

${platform_sdk_ver_str}
${apps_sdk_ver_str}

This container image and its contents are governed by the terms of
the license at /usr/share/doc/CONTAINER_LICENSE.txt.  By pulling and
using the container, you accept the terms and conditions of this license.

EOM
