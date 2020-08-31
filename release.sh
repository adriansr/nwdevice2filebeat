#!/bin/bash

if test $# -ne 2
then
    echo "Usage: $0 <release.csv> <path-to-beats-repo>" >&2
    exit 1
fi

BATCH="$1"
BEATS_DIR="$2"
OUTPUT=generated.output
ARGS="-F whitespace -O deduplicate,globals"
BASE_PORT=9522
FILESETS=""
DO_MODULE=${DO_MODULE:-1}
DO_PACKAGE=${DO_PACKAGE:-1}

die() {
    logerr "$@"
    exit 1
}

logerr() {
    echo "$0: $*" >&2
}

convert_device() {
    ROW="$1"
    DEV=$(echo $ROW | cut -d, -f1)
    MOD=$(echo $ROW | cut -d, -f2)
    FST=$(echo $ROW | cut -d, -f3)
    VENDOR=$(echo $ROW | cut -d, -f4)
    PROD=$(echo $ROW | cut -d, -f5)
    TYPE=$(echo $ROW | cut -d, -f6)
    VERSION=$(echo $ROW | cut -d, -f7)
    CATEGORIES=$(echo $ROW | cut -d, -f9 | tr ';' ,)
    PORT=$(expr $BASE_PORT + $LINE)
    DIR=$OUTPUT/$DEV
    PRODINFO="--vendor $VENDOR --product $PROD --type $TYPE"
    test -d devices/$DEV || die "Device $DEV does not exists"
    mkdir -p $DIR/module $DIR/package
    if test "$DO_MODULE" = "1"
    then
        echo "$DEV: generating module [$LINE/$LINES]"
        echo "$DEV: generating module [$LINE/$LINES]" >> $DIR/output.log 2>&1
        ./nwdevice2filebeat generate module --device devices/$DEV/ $ARGS $PRODINFO --output $DIR/module --port $PORT --module $MOD --fileset $FST >> $DIR/output.log 2>&1 || die "Failed generating module"
    fi
    echo "$DEV: generating package [$LINE/$LINES]"
    echo "$DEV: generating package [$LINE/$LINES]" >> $DIR/output.log 2>&1
    ./nwdevice2filebeat generate package --device devices/$DEV/ $ARGS $PRODINFO --output $DIR/package --port $PORT \
        --module $MOD --fileset $FST --categories "$CATEGORIES" --version $VERSION >> $DIR/output.log 2>&1 || die "Failed generating package"
    if test "$DO_MODULE" = "1"
    then
        echo "$DEV: generating logs [$LINE/$LINES]"
        echo "$DEV: generating logs [$LINE/$LINES]" >> $DIR/output.log 2>&1
        mkdir $DIR/module/$MOD/$FST/test
        ./nwdevice2filebeat generate logs --device devices/$DEV $ARGS --output $DIR/module/$MOD/$FST/test/generated.log --lines 100 >> $DIR/output.log 2>&1
        if test $? -eq 0
        then
            echo "$DEV: testing logs [$LINE/$LINES]"
            echo "$DEV: testing logs [$LINE/$LINES]" >> $DIR/output.log 2>&1
            ./nwdevice2filebeat run --device devices/$DEV $ARGS --logs $DIR/module/$MOD/$FST/test/generated.log >> $DIR/output.log 2>&1 || logerr "Failed testing logs"
        else
            logerr "Failed generating logs for $DEV"
        fi
        for log_file in $(ls samples/$DEV/*.log 2>/dev/null)
        do
            cp $log_file $DIR/module/$MOD/$FST/test/
        done
    fi
}

module_to_fb() {
    ROW="$1"
    DEV=$(echo $ROW | cut -d, -f1)
    MOD=$(echo $ROW | cut -d, -f2)
    FST=$(echo $ROW | cut -d, -f3)

    MODULES_DIR="$BEATS_DIR/x-pack/filebeat/module"
    if test -d "$MODULES_DIR/$MOD"
    then
        # Merge in existing module
        test -d "$MODULES_DIR/$MOD/$FST" && die "fileset $MOD/$FST already exists!"
        cp -a "$OUTPUT/$DEV/module/$MOD/$FST" "$MODULES_DIR/$MOD/" || die "Failed copying fileset $MOD/$FST for $DEV"
        
        # Merge config.yml
        (cat "$OUTPUT/$DEV/module/$MOD/_meta/config.yml" | sed 's/^- module:.*//' >> "$MODULES_DIR/$MOD/_meta/config.yml") || die "Failed merging config.yml"
        echo "Copied fileset $MOD/$FST"

        # Merge docs
        SRC="$OUTPUT/$DEV/module/$MOD/_meta/docs.asciidoc"
        DST="$MODULES_DIR/$MOD/_meta/docs.asciidoc"
        merge_docs "$SRC" "$DST" || die "Failed merging docs"
    else
        # Copy new module
        cp -a "$OUTPUT/$DEV/module/$MOD" "$MODULES_DIR/" || die "Failed copying mod=$MOD for $DEV"
        echo "Copied module $MOD"
    fi
    for test_file in $(ls "$MODULES_DIR/$MOD/$FST/test/"*.log 2> /dev/null)
    do
        touch "$test_file"-expected.json
    done
}

merge_docs() {
    SRC="$1"
    DST="$2"
    SRC_FROM_LINE=$(grep -n '^==== .* fileset settings' "$SRC" | cut -d: -f1)
    SRC_TO_LINE=$(grep -n ':fileset_ex!:' "$SRC" | cut -d: -f1)
    SRC_TO_LINE=$(expr $SRC_TO_LINE + 1)
    test "$(echo $SRC_FROM_LINE | wc -l)" -eq 1 -a "$(echo $SRC_TO_LINE | wc -l)" -eq 1 || die "Unable to extract fileset from docs"
    SRC_FROM_LINE=$(expr $SRC_FROM_LINE - 1)
    SRC_NUM_LINES=$(expr $SRC_TO_LINE - $SRC_FROM_LINE + 1)
    DST_LINE=$(grep -n ':fileset_ex!:' "$DST" | tail -1 | cut -d: -f1)
    test "$DST_LINE" = "" && die "Unable to find last fileset in target docs"
    DST_LINE=$(expr $DST_LINE + 1)
    DST_NUM_LINES=$(expr $(cat "$DST" | wc -l) + 0)
    head -$DST_LINE "$DST" > "$DST".tmp
    head -$SRC_TO_LINE "$SRC" | tail -$SRC_NUM_LINES >> "$DST".tmp
    tail -$(expr $DST_NUM_LINES - $DST_LINE) "$DST" >> "$DST".tmp
    mv "$DST".tmp "$DST"
}

test_fileset() {
    ROW="$1"
    DEV=$(echo $ROW | cut -d, -f1)
    MOD=$(echo $ROW | cut -d, -f2)
    FST=$(echo $ROW | cut -d, -f3)
    
    FILES="$(ls $OUTPUT/$DEV/module/$MOD/$FST/test/*.log 2>/dev/null)"
    if test -z "$FILES"
    then
        echo ""
        echo "No test files in $MOD/$FST"
        return 0
    fi

    pushd "$BEATS_DIR/x-pack/filebeat"
    
    echo ""
    echo "Generating golden files for $MOD/$FST [$LINE/$LINES]"
    MODULES_PATH="$BEATS_DIR/x-pack/filebeat/module" \
        INTEGRATION_TESTS=1                          \
        TESTING_FILEBEAT_MODULES=$MOD                \
        TESTING_FILEBEAT_FILESETS=$FST               \
        GENERATE=1                                   \
        nosetests -v -s tests/system/test_xpack_modules.py || die "Generating golden files failed"

    echo ""
    echo "Testing $MOD/$FST [$LINE/$LINES]"
    MODULES_PATH="$BEATS_DIR/x-pack/filebeat/module" \
        INTEGRATION_TESTS=1                          \
        TESTING_FILEBEAT_MODULES=$MOD                \
        TESTING_FILEBEAT_FILESETS=$FST               \
        nosetests -v -s tests/system/test_xpack_modules.py || die "Generating golden files failed"

    popd
}

foreach_line() {
    FILE="$1"
    ACT="$2"
    test -f "$FILE" || die "No such file $FILE"
    LINES=$(cat "$FILE" | wc -l)
    LINES=$(expr $LINES + 0)
    test "$LINES" -gt 0 || die "Cannot read $FILE"
    LINE=0
    while test $LINE -lt $LINES
    do
        LINE=$(expr $LINE + 1)
        ROW=$(head -$LINE "$FILE" | tail -1)
        $ACT "$ROW"
    done
}

if test $DO_MODULE -eq 1
then
    test -d "$BEATS_DIR/libbeat" -a "$BEATS_DIR/.git" || die "Beats dir is not the beats repo"
    echo "This will destroy all your changes in Beats repo ($BEATS_DIR)."
    read -p "Continue? [y/n] " ANS
    test $ANS = 'y' -o $ANS = 'Y' -o $ANS = 'yes' || die "cancelled by user."
fi


if test -d venv
then
    . venv/bin/activate
else
    python3 -mvenv venv || die "failed creating virtualenv"
    . venv/bin/activate
    pip install -r "$BEATS_DIR/libbeat/tests/system/requirements.txt" || die "Failed installing test requirements"
fi

rm -rf $OUTPUT
foreach_line "$BATCH" convert_device

if [ "$DO_MODULE" = 1 ]
then
    echo ''
    echo 'Cleaning up beats repo'
    pushd "$BEATS_DIR" && git clean -df && git reset --hard && popd || die "Failed cleaning beats repo"

    foreach_line "$BATCH" module_to_fb

    echo ''
    echo 'Running mage update'
    pushd "$BEATS_DIR/x-pack/filebeat" && mage update && popd || die "Failed mage update"

    echo ''
    echo 'Building filebeat'
    pushd "$BEATS_DIR/x-pack/filebeat" && mage build && go test -c && popd || die "Failed mage update"


    echo ''
    echo 'Validating fields'
    pushd "$BEATS_DIR/x-pack/filebeat" && ./filebeat export index-pattern > /dev/null && popd || die "Index-pattern export failed: Broken fields!"
    pushd "$BEATS_DIR/x-pack/filebeat" && ./filebeat export template > /dev/null && popd || die "Template export failed: Broken fields!"

    foreach_line "$BATCH" test_fileset

    echo ''
    echo 'Updating filebeat docs'
    pushd "$BEATS_DIR/filebeat" && make update && popd || die "Failed cleaning beats repo"

fi
