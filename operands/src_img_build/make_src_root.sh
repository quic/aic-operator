#!/bin/bash
# -----------------------------------------------------------------------------
#
# Copyright (c) Qualcomm Technologies, Inc. and/or its subsidiaries. All rights reserved.
# Confidential and Proprietary - Qualcomm Technologies, Inc. and/or its subsidiaries.
#
# -----------------------------------------------------------------------------

BUILD_TYPE=""
TOOLS_NEEDED=""
FW_INPUT=$1
KMD_INPUT=$2

SCRIPT_DIR=$(dirname "$(readlink -f "$0")")

if [ $# != 2 ] ; then
  echo "usage:"
  echo "./make_src_root.sh qaic-fw.pkg qaic-kmd.pkg"
  exit 255
fi

if [ "${FW_INPUT:(-3)}" != "${KMD_INPUT:(-3)}" ] ; then
  echo "$FW_INPUT and $KMD_INPUT do not share the same file extension!"
  exit 255
elif [ "${FW_INPUT:(-3)}" == "deb" ] ; then
  BUILD_TYPE="deb"
  TOOLS_NEEDED=( "ar" "tar" )
elif [ "${FW_INPUT:(-3)}" == "rpm" ] ; then
  BUILD_TYPE="rpm"
  TOOLS_NEEDED=( "rpm2cpio" "cpio" )
else
  echo "unknown file extension ${FW_INPUT:(-3)}"
  exit 255
fi
FW_PKG="$(readlink -f "$FW_INPUT")"
KMD_PKG="$(readlink -f "$KMD_INPUT")"

if [[ ! "$FW_PKG" =~ "qaic-fw" ]] ; then
  echo "$FW_INPUT doesn't match 'qaic-fw*'"
  exit 255
fi

if [[ ! "$KMD_PKG" =~ "qaic-kmd" ]] ; then
  echo "$KMD_INPUT doesn't match 'qaic-kmd*'"
  exit 255
fi

if [ ! -f "$FW_PKG" ] || [ ! -f "$KMD_PKG" ] ; then
  echo "no such files: $FW_PKG, $KMD_PKG"
  exit 255
fi


if ! command -v ${TOOLS_NEEDED[0]} &> /dev/null || ! command -v ${TOOLS_NEEDED[1]} &> /dev/null ; then
  echo "cannot support; host does not have necessary tools:"
  echo "${TOOLS_NEEDED[*]}"
  exit 255
fi

SRC_ROOTDIR=$(pwd)/src_rootdir
mkdir -p "$SRC_ROOTDIR"/{firmware,src}
TMP_DIR=$(pwd)/tmp_dir
mkdir "$TMP_DIR"

cp $FW_PKG $TMP_DIR
cp $KMD_PKG $TMP_DIR

pushd $TMP_DIR &> /dev/null
case ${BUILD_TYPE} in
"deb")
  ar x $FW_PKG && tar -xf data.tar.gz -C ./
  ar x $KMD_PKG && tar -xf data.tar.xz -C ./
  echo "extraction complete to $SRC_ROOTDIR"
;;
"rpm")
  rpm2cpio $FW_PKG | cpio -idmv
  rpm2cpio $KMD_PKG | cpio -idmv
  echo "extraction complete to $SRC_ROOTDIR"
;;
*)
  echo "invalid build-type = ${BUILD_TYPE}"
  rmdir $SRC_ROOTDIR
  popd &> /dev/null
  rm -rf $TMP_DIR
  exit 255
;;
esac
cp -r ./lib/firmware/* $SRC_ROOTDIR/firmware
cp -r ./opt/qti-aic/firmware/fw2_swe.json $SRC_ROOTDIR/firmware
cp -r ./usr/src/qaic-*/* $SRC_ROOTDIR/src
popd &> /dev/null
rm -rf $TMP_DIR


