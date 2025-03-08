package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"os"
)

var rpmCodes = map[int32]string{
	1000: "rpmTagPackage",
	1001: "rpmTagVersion",
	1002: "rpmTagRelease",
	1004: "rpmTagSummary",
	1005: "rpmTagDesc",
	1006: "rpmTagBuildTime",
	1007: "rpmTagBuildHost",
	1009: "rpmTagSize",
	1010: "rpmTagDistrib",
	1011: "rpmTagVendor",
	1014: "rpmTagLicense",
	1015: "rpmTagPackager",
	1016: "rpmTagGroup",
	1020: "rpmTagURL",
	1021: "rpmTagOS",
	1022: "rpmTagArch",
	1124: "rpmTagPayload",
	1125: "rpmTagCompressor",
	1126: "rpmTagPayloadFlags",
	1141: "rpmTagFileClass",
	1028: "rpmTagFileSizes",
	1030: "rpmTagFileModes",
	1033: "rpmTagFileDevs",
	1034: "rpmTagFileTimes",
	1035: "rpmTagFileDigests",
	1036: "rpmTagFileLinks",
	1037: "rpmTagFileFlags",
	1039: "rpmTagOwners",
	1040: "rpmTagGroups",
	1096: "rpmTagFileInodes",
	1097: "rpmTagFileLangs",
	1116: "rpmTagDirIndexes",
	1117: "rpmTagBasenames",
	1118: "rpmTagDirnames",
	1080: "rpmTagChangeTime",
	1081: "rpmTagChangeName",
	1082: "rpmTagChangeText",
	1047: "rpmTagProvideName",
	1113: "rpmTagProvideVersion",
	1112: "rpmTagProvideFlags",
	1049: "rpmTagRequireName",
	1050: "rpmTagRequireVersion",
	1048: "rpmTagRequireFlags",
	1054: "rpmTagConflictName",
	1055: "rpmTagConflictVersion",
	1053: "rpmTagConflictFlags",
	1064: "rpmTagRpmVersion",
	1023: "rpmTagPrein",
	5020: "rpmTagPreinFlags",
	1085: "rpmTagPreinProg",
	1024: "rpmTagPostin",
	5021: "rpmTagPostinFlags",
	1086: "rpmTagPostinProg",
	1026: "rpmTagPostun",
	5023: "rpmTagPostunFlags",
	1088: "rpmTagPostunProg",
	1025: "rpmTagPreun",
	5022: "rpmTagPreunFlags",
	1087: "rpmTagPreunProg",
	1079: "rpmTagCheckScript",
	5026: "rpmTagCheckScriptFlags",
	1091: "rpmTagCheckScriptProg",
	1151: "rpmTagTrans",
	5024: "rpmTagTransFlags",
	1153: "rpmTagTransProg",
	1090: "rpmTagObsoleteName",
	1115: "rpmTagObsoleteVersion",
	1114: "rpmTagObsoleteFlags",
	5055: "rpmTagEnhanceName",
	5056: "rpmTagEnhanceVersion",
	5057: "rpmTagEnhanceFlags",
	5046: "rpmTagRecommendName",
	5047: "rpmTagRecommendVersion",
	5048: "rpmTagRecommendFlags",
	5049: "rpmTagSuggestName",
	5050: "rpmTagSuggestVersion",
	5051: "rpmTagSuggestFlags",
	1132: "rpmTagPlatform",
	5011: "rpmTagFileDigestAlgo",
	5001: "rpmTagFileProvide",
	5002: "rpmTagFileRequire",
}

func main() {
	flag.Parse()

	r, err := os.Open(flag.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer r.Close()

	rs := bufio.NewReader(r)
	rs.Discard(16 * 6)

	readStore("signature", rs, true)
	readStore("package", rs, false)
}

func readStore(name string, rs *bufio.Reader, padded bool) {
	rs.Discard(8)
	var (
		count int32
		size  int32
		index bytes.Buffer
		tmp   bytes.Buffer
	)
	binary.Read(rs, binary.BigEndian, &count)
	binary.Read(rs, binary.BigEndian, &size)

	fmt.Println(name, count, size)
	io.CopyN(&index, rs, int64(count*16))
	io.CopyN(&tmp, rs, int64(size))

	store := bytes.NewReader(tmp.Bytes())
	for i := 0; i < int(count); i++ {
		readElement(&index, store)
	}
	size += 16
	if mod := size % 8; padded && mod != 0 {
		rs.Discard(int(8 - mod))
	}
}

func readElement(index io.Reader, store io.ReadSeeker) {
	var (
		tag    int32
		kind   int32
		offset int32
		size   int32
	)
	binary.Read(index, binary.BigEndian, &tag)
	binary.Read(index, binary.BigEndian, &kind)
	binary.Read(index, binary.BigEndian, &offset)
	binary.Read(index, binary.BigEndian, &size)

	name, ok := rpmCodes[tag]
	if !ok {
		name = "unknown!!!"
	}
	fmt.Printf(">>> %s(%d)\n", name, tag)
	store.Seek(int64(offset), io.SeekStart)
	if kind == 6 || kind == 8 || kind == 9 {
		rs := bufio.NewReader(store)
		for i := 0; i < int(size); i++ {
			str, _ := rs.ReadString(0)
			fmt.Println("*", i+1, str)
		}
	} else if kind == 3 {
		var val int16
		for i := 0; i < int(size); i++ {
			binary.Read(store, binary.BigEndian, &val)
			fmt.Println("* i16:", i+1, val)
		}
	} else if kind == 4 {
		var val int32
		for i := 0; i < int(size); i++ {
			binary.Read(store, binary.BigEndian, &val)
			fmt.Println("* i32:", i+1, val)
		}
	}
}
