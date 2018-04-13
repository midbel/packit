package rpm

type Tag int32

func (t Tag) String() string {
	v, ok := tags[t]
	if !ok {
		return "unknown"
	}
	return v
}

const (
	TagNAME                        = 1000
	TagVERSION                     = 1001
	TagRELEASE                     = 1002
	TagEPOCH                       = 1003
	TagSUMMARY                     = 1004
	TagDESCRIPTION                 = 1005
	TagBUILDTIME                   = 1006
	TagBUILDHOST                   = 1007
	TagINSTALLTIME                 = 1008
	TagSIZE                        = 1009
	TagDISTRIBUTION                = 1010
	TagVENDOR                      = 1011
	TagGIF                         = 1012
	TagXPM                         = 1013
	TagLICENSE                     = 1014
	TagPACKAGER                    = 1015
	TagGROUP                       = 1016
	TagCHANGELOG                   = 1017
	TagSOURCE                      = 1018
	TagPATCH                       = 1019
	TagURL                         = 1020
	TagOS                          = 1021
	TagARCH                        = 1022
	TagPREIN                       = 1023
	TagPOSTIN                      = 1024
	TagPREUN                       = 1025
	TagPOSTUN                      = 1026
	TagOLDFILENAMES                = 1027
	TagFILESIZES                   = 1028
	TagFILESTATES                  = 1029
	TagFILEMODES                   = 1030
	TagFILEUIDS                    = 1031
	TagFILEGIDS                    = 1032
	TagFILERDEVS                   = 1033
	TagFILEMTIMES                  = 1034
	TagFILEDIGESTS                 = 1035
	TagFILELINKTOS                 = 1036
	TagFILEFLAGS                   = 1037
	TagROOT                        = 1038
	TagFILEUSERNAME                = 1039
	TagFILEGROUPNAME               = 1040
	TagEXCLUDE                     = 1041
	TagEXCLUSIVE                   = 1042
	TagICON                        = 1043
	TagSOURCERPM                   = 1044
	TagFILEVERIFYFLAGS             = 1045
	TagARCHIVESIZE                 = 1046
	TagPROVIDENAME                 = 1047
	TagREQUIREFLAGS                = 1048
	TagREQUIRENAME                 = 1049
	TagREQUIREVERSION              = 1050
	TagNOSOURCE                    = 1051
	TagNOPATCH                     = 1052
	TagCONFLICTFLAGS               = 1053
	TagCONFLICTNAME                = 1054
	TagCONFLICTVERSION             = 1055
	TagDEFAULTPREFIX               = 1056
	TagBUILDROOT                   = 1057
	TagINSTALLPREFIX               = 1058
	TagEXCLUDEARCH                 = 1059
	TagEXCLUDEOS                   = 1060
	TagEXCLUSIVEARCH               = 1061
	TagEXCLUSIVEOS                 = 1062
	TagAUTOREQPROV                 = 1063
	TagRPMVERSION                  = 1064
	TagTRIGGERSCRIPTS              = 1065
	TagTRIGGERNAME                 = 1066
	TagTRIGGERVERSION              = 1067
	TagTRIGGERFLAGS                = 1068
	TagTRIGGERINDEX                = 1069
	TagVERIFYSCRIPT                = 1079
	TagCHANGELOGTIME               = 1080
	TagCHANGELOGNAME               = 1081
	TagCHANGELOGTEXT               = 1082
	TagBROKENMD5                   = 1083
	TagPREREQ                      = 1084
	TagPREINPROG                   = 1085
	TagPOSTINPROG                  = 1086
	TagPREUNPROG                   = 1087
	TagPOSTUNPROG                  = 1088
	TagBUILDARCHS                  = 1089
	TagOBSOLETENAME                = 1090
	TagVERIFYSCRIPTPROG            = 1091
	TagTRIGGERSCRIPTPROG           = 1092
	TagDOCDIR                      = 1093
	TagCOOKIE                      = 1094
	TagFILEDEVICES                 = 1095
	TagFILEINODES                  = 1096
	TagFILELANGS                   = 1097
	TagPREFIXES                    = 1098
	TagINSTPREFIXES                = 1099
	TagTRIGGERIN                   = 1100
	TagTRIGGERUN                   = 1101
	TagTRIGGERPOSTUN               = 1102
	TagAUTOREQ                     = 1103
	TagAUTOPROV                    = 1104
	TagCAPABILITY                  = 1105
	TagSOURCEPACKAGE               = 1106
	TagOLDORIGFILENAMES            = 1107
	TagBUILDPREREQ                 = 1108
	TagBUILDREQUIRES               = 1109
	TagBUILDCONFLICTS              = 1110
	TagBUILDMACROS                 = 1111
	TagPROVIDEFLAGS                = 1112
	TagPROVIDEVERSION              = 1113
	TagOBSOLETEFLAGS               = 1114
	TagOBSOLETEVERSION             = 1115
	TagDIRINDEXES                  = 1116
	TagBASENAMES                   = 1117
	TagDIRNAMES                    = 1118
	TagORIGDIRINDEXES              = 1119
	TagORIGBASENAMES               = 1120
	TagORIGDIRNAMES                = 1121
	TagOPTFLAGS                    = 1122
	TagDISTURL                     = 1123
	TagPAYLOADFORMAT               = 1124
	TagPAYLOADCOMPRESSOR           = 1125
	TagPAYLOADFLAGS                = 1126
	TagINSTALLCOLOR                = 1127
	TagINSTALLTID                  = 1128
	TagREMOVETID                   = 1129
	TagSHA1RHN                     = 1130
	TagRHNPLATFORM                 = 1131
	TagPLATFORM                    = 1132
	TagPATCHESNAME                 = 1133
	TagPATCHESFLAGS                = 1134
	TagPATCHESVERSION              = 1135
	TagCACHECTIME                  = 1136
	TagCACHEPKGPATH                = 1137
	TagCACHEPKGSIZE                = 1138
	TagCACHEPKGMTIME               = 1139
	TagFILECOLORS                  = 1140
	TagFILECLASS                   = 1141
	TagCLASSDICT                   = 1142
	TagFILEDEPENDSX                = 1143
	TagFILEDEPENDSN                = 1144
	TagDEPENDSDICT                 = 1145
	TagSOURCEPKGID                 = 1146
	TagFILECONTEXTS                = 1147
	TagFSCONTEXTS                  = 1148
	TagRECONTEXTS                  = 1149
	TagPOLICIES                    = 1150
	TagPRETRANS                    = 1151
	TagPOSTTRANS                   = 1152
	TagPRETRANSPROG                = 1153
	TagPOSTTRANSPROG               = 1154
	TagDISTTAG                     = 1155
	TagOLDSUGGESTSNAME             = 1156
	TagOLDSUGGESTSVERSION          = 1157
	TagOLDSUGGESTSFLAGS            = 1158
	TagOLDENHANCESNAME             = 1159
	TagOLDENHANCESVERSION          = 1160
	TagOLDENHANCESFLAGS            = 1161
	TagPRIORITY                    = 1162
	TagCVSID                       = 1163
	TagBLINKPKGID                  = 1164
	TagBLINKHDRID                  = 1165
	TagBLINKNEVRA                  = 1166
	TagFLINKPKGID                  = 1167
	TagFLINKHDRID                  = 1168
	TagFLINKNEVRA                  = 1169
	TagPACKAGEORIGIN               = 1170
	TagTRIGGERPREIN                = 1171
	TagBUILDSUGGESTS               = 1172
	TagBUILDENHANCES               = 1173
	TagSCRIPTSTATES                = 1174
	TagSCRIPTMETRICS               = 1175
	TagBUILDCPUCLOCK               = 1176
	TagFILEDIGESTALGOS             = 1177
	TagVARIANTS                    = 1178
	TagXMAJOR                      = 1179
	TagXMINOR                      = 1180
	TagREPOTAG                     = 1181
	TagKEYWORDS                    = 1182
	TagBUILDPLATFORMS              = 1183
	TagPACKAGECOLOR                = 1184
	TagPACKAGEPREFCOLOR            = 1185
	TagXATTRSDICT                  = 1186
	TagFILEXATTRSX                 = 1187
	TagDEPATTRSDICT                = 1188
	TagCONFLICTATTRSX              = 1189
	TagOBSOLETEATTRSX              = 1190
	TagPROVIDEATTRSX               = 1191
	TagREQUIREATTRSX               = 1192
	TagBUILDPROVIDES               = 1193
	TagBUILDOBSOLETES              = 1194
	TagDBINSTANCE                  = 1195
	TagNVRA                        = 1196
	TagFILENAMES                   = 5000
	TagFILEPROVIDE                 = 5001
	TagFILEREQUIRE                 = 5002
	TagFSNAMES                     = 5003
	TagFSSIZES                     = 5004
	TagTRIGGERCONDS                = 5005
	TagTRIGGERTYPE                 = 5006
	TagORIGFILENAMES               = 5007
	TagLONGFILESIZES               = 5008
	TagLONGSIZE                    = 5009
	TagFILECAPS                    = 5010
	TagFILEDIGESTALGO              = 5011
	TagBUGURL                      = 5012
	TagEVR                         = 5013
	TagNVR                         = 5014
	TagNEVR                        = 5015
	TagNEVRA                       = 5016
	TagHEADERCOLOR                 = 5017
	TagVERBOSE                     = 5018
	TagEPOCHNUM                    = 5019
	TagPREINFLAGS                  = 5020
	TagPOSTINFLAGS                 = 5021
	TagPREUNFLAGS                  = 5022
	TagPOSTUNFLAGS                 = 5023
	TagPRETRANSFLAGS               = 5024
	TagPOSTTRANSFLAGS              = 5025
	TagVERIFYSCRIPTFLAGS           = 5026
	TagTRIGGERSCRIPTFLAGS          = 5027
	TagCOLLECTIONS                 = 5029
	TagPOLICYNAMES                 = 5030
	TagPOLICYTYPES                 = 5031
	TagPOLICYTYPESINDEXES          = 5032
	TagPOLICYFLAGS                 = 5033
	TagVCS                         = 5034
	TagORDERNAME                   = 5035
	TagORDERVERSION                = 5036
	TagORDERFLAGS                  = 5037
	TagMSSFMANIFEST                = 5038
	TagMSSFDOMAIN                  = 5039
	TagINSTFILENAMES               = 5040
	TagREQUIRENEVRS                = 5041
	TagPROVIDENEVRS                = 5042
	TagOBSOLETENEVRS               = 5043
	TagCONFLICTNEVRS               = 5044
	TagFILENLINKS                  = 5045
	TagRECOMMENDNAME               = 5046
	TagRECOMMENDVERSION            = 5047
	TagRECOMMENDFLAGS              = 5048
	TagSUGGESTNAME                 = 5049
	TagSUGGESTVERSION              = 5050
	TagSUGGESTFLAGS                = 5051
	TagSUPPLEMENTNAME              = 5052
	TagSUPPLEMENTVERSION           = 5053
	TagSUPPLEMENTFLAGS             = 5054
	TagENHANCENAME                 = 5055
	TagENHANCEVERSION              = 5056
	TagENHANCEFLAGS                = 5057
	TagRECOMMENDNEVRS              = 5058
	TagSUGGESTNEVRS                = 5059
	TagSUPPLEMENTNEVRS             = 5060
	TagENHANCENEVRS                = 5061
	TagENCODING                    = 5062
	TagFILETRIGGERIN               = 5063
	TagFILETRIGGERUN               = 5064
	TagFILETRIGGERPOSTUN           = 5065
	TagFILETRIGGERSCRIPTS          = 5066
	TagFILETRIGGERSCRIPTPROG       = 5067
	TagFILETRIGGERSCRIPTFLAGS      = 5068
	TagFILETRIGGERNAME             = 5069
	TagFILETRIGGERINDEX            = 5070
	TagFILETRIGGERVERSION          = 5071
	TagFILETRIGGERFLAGS            = 5072
	TagTRANSFILETRIGGERIN          = 5073
	TagTRANSFILETRIGGERUN          = 5074
	TagTRANSFILETRIGGERPOSTUN      = 5075
	TagTRANSFILETRIGGERSCRIPTS     = 5076
	TagTRANSFILETRIGGERSCRIPTPROG  = 5077
	TagTRANSFILETRIGGERSCRIPTFLAGS = 5078
	TagTRANSFILETRIGGERNAME        = 5079
	TagTRANSFILETRIGGERINDEX       = 5080
	TagTRANSFILETRIGGERVERSION     = 5081
	TagTRANSFILETRIGGERFLAGS       = 5082
	TagREMOVEPATHPOSTFIXES         = 5083
	TagFILETRIGGERPRIORITIES       = 5084
	TagTRANSFILETRIGGERPRIORITIES  = 5085
	TagFILETRIGGERCONDS            = 5086
	TagFILETRIGGERTYPE             = 5087
	TagTRANSFILETRIGGERCONDS       = 5088
	TagTRANSFILETRIGGERTYPE        = 5089
	TagFILESIGNATURES              = 5090
	TagFILESIGNATURELENGTH         = 5091
	TagPAYLOADDIGEST               = 5092
	TagPAYLOADDIGESTALGO           = 5093
	TagAUTOINSTALLED               = 5094
	TagIDENTITY                    = 5095
)

var tags = map[Tag]string{
	TagNAME:                        "NAME",
	TagVERSION:                     "VERSION",
	TagRELEASE:                     "RELEASE",
	TagEPOCH:                       "EPOCH",
	TagSUMMARY:                     "SUMMARY",
	TagDESCRIPTION:                 "DESCRIPTION",
	TagBUILDTIME:                   "BUILDTIME",
	TagBUILDHOST:                   "BUILDHOST",
	TagINSTALLTIME:                 "INSTALLTIME",
	TagSIZE:                        "SIZE",
	TagDISTRIBUTION:                "DISTRIBUTION",
	TagVENDOR:                      "VENDOR",
	TagGIF:                         "GIF",
	TagXPM:                         "XPM",
	TagLICENSE:                     "LICENSE",
	TagPACKAGER:                    "PACKAGER",
	TagGROUP:                       "GROUP",
	TagCHANGELOG:                   "CHANGELOG",
	TagSOURCE:                      "SOURCE",
	TagPATCH:                       "PATCH",
	TagURL:                         "URL",
	TagOS:                          "OS",
	TagARCH:                        "ARCH",
	TagPREIN:                       "PREIN",
	TagPOSTIN:                      "POSTIN",
	TagPREUN:                       "PREUN",
	TagPOSTUN:                      "POSTUN",
	TagOLDFILENAMES:                "OLDFILENAMES",
	TagFILESIZES:                   "FILESIZES",
	TagFILESTATES:                  "FILESTATES",
	TagFILEMODES:                   "FILEMODES",
	TagFILEUIDS:                    "FILEUIDS",
	TagFILEGIDS:                    "FILEGIDS",
	TagFILERDEVS:                   "FILERDEVS",
	TagFILEMTIMES:                  "FILEMTIMES",
	TagFILEDIGESTS:                 "FILEDIGESTS",
	TagFILELINKTOS:                 "FILELINKTOS",
	TagFILEFLAGS:                   "FILEFLAGS",
	TagROOT:                        "ROOT",
	TagFILEUSERNAME:                "FILEUSERNAME",
	TagFILEGROUPNAME:               "FILEGROUPNAME",
	TagEXCLUDE:                     "EXCLUDE",
	TagEXCLUSIVE:                   "EXCLUSIVE",
	TagICON:                        "ICON",
	TagSOURCERPM:                   "SOURCERPM",
	TagFILEVERIFYFLAGS:             "FILEVERIFYFLAGS",
	TagARCHIVESIZE:                 "ARCHIVESIZE",
	TagPROVIDENAME:                 "PROVIDENAME",
	TagREQUIREFLAGS:                "REQUIREFLAGS",
	TagREQUIRENAME:                 "REQUIRENAME",
	TagREQUIREVERSION:              "REQUIREVERSION",
	TagNOSOURCE:                    "NOSOURCE",
	TagNOPATCH:                     "NOPATCH",
	TagCONFLICTFLAGS:               "CONFLICTFLAGS",
	TagCONFLICTNAME:                "CONFLICTNAME",
	TagCONFLICTVERSION:             "CONFLICTVERSION",
	TagDEFAULTPREFIX:               "DEFAULTPREFIX",
	TagBUILDROOT:                   "BUILDROOT",
	TagINSTALLPREFIX:               "INSTALLPREFIX",
	TagEXCLUDEARCH:                 "EXCLUDEARCH",
	TagEXCLUDEOS:                   "EXCLUDEOS",
	TagEXCLUSIVEARCH:               "EXCLUSIVEARCH",
	TagEXCLUSIVEOS:                 "EXCLUSIVEOS",
	TagAUTOREQPROV:                 "AUTOREQPROV",
	TagRPMVERSION:                  "RPMVERSION",
	TagTRIGGERSCRIPTS:              "TRIGGERSCRIPTS",
	TagTRIGGERNAME:                 "TRIGGERNAME",
	TagTRIGGERVERSION:              "TRIGGERVERSION",
	TagTRIGGERFLAGS:                "TRIGGERFLAGS",
	TagTRIGGERINDEX:                "TRIGGERINDEX",
	TagVERIFYSCRIPT:                "VERIFYSCRIPT",
	TagCHANGELOGTIME:               "CHANGELOGTIME",
	TagCHANGELOGNAME:               "CHANGELOGNAME",
	TagCHANGELOGTEXT:               "CHANGELOGTEXT",
	TagBROKENMD5:                   "BROKENMD5",
	TagPREREQ:                      "PREREQ",
	TagPREINPROG:                   "PREINPROG",
	TagPOSTINPROG:                  "POSTINPROG",
	TagPREUNPROG:                   "PREUNPROG",
	TagPOSTUNPROG:                  "POSTUNPROG",
	TagBUILDARCHS:                  "BUILDARCHS",
	TagOBSOLETENAME:                "OBSOLETENAME",
	TagVERIFYSCRIPTPROG:            "VERIFYSCRIPTPROG",
	TagTRIGGERSCRIPTPROG:           "TRIGGERSCRIPTPROG",
	TagDOCDIR:                      "DOCDIR",
	TagCOOKIE:                      "COOKIE",
	TagFILEDEVICES:                 "FILEDEVICES",
	TagFILEINODES:                  "FILEINODES",
	TagFILELANGS:                   "FILELANGS",
	TagPREFIXES:                    "PREFIXES",
	TagINSTPREFIXES:                "INSTPREFIXES",
	TagTRIGGERIN:                   "TRIGGERIN",
	TagTRIGGERUN:                   "TRIGGERUN",
	TagTRIGGERPOSTUN:               "TRIGGERPOSTUN",
	TagAUTOREQ:                     "AUTOREQ",
	TagAUTOPROV:                    "AUTOPROV",
	TagCAPABILITY:                  "CAPABILITY",
	TagSOURCEPACKAGE:               "SOURCEPACKAGE",
	TagOLDORIGFILENAMES:            "OLDORIGFILENAMES",
	TagBUILDPREREQ:                 "BUILDPREREQ",
	TagBUILDREQUIRES:               "BUILDREQUIRES",
	TagBUILDCONFLICTS:              "BUILDCONFLICTS",
	TagBUILDMACROS:                 "BUILDMACROS",
	TagPROVIDEFLAGS:                "PROVIDEFLAGS",
	TagPROVIDEVERSION:              "PROVIDEVERSION",
	TagOBSOLETEFLAGS:               "OBSOLETEFLAGS",
	TagOBSOLETEVERSION:             "OBSOLETEVERSION",
	TagDIRINDEXES:                  "DIRINDEXES",
	TagBASENAMES:                   "BASENAMES",
	TagDIRNAMES:                    "DIRNAMES",
	TagORIGDIRINDEXES:              "ORIGDIRINDEXES",
	TagORIGBASENAMES:               "ORIGBASENAMES",
	TagORIGDIRNAMES:                "ORIGDIRNAMES",
	TagOPTFLAGS:                    "OPTFLAGS",
	TagDISTURL:                     "DISTURL",
	TagPAYLOADFORMAT:               "PAYLOADFORMAT",
	TagPAYLOADCOMPRESSOR:           "PAYLOADCOMPRESSOR",
	TagPAYLOADFLAGS:                "PAYLOADFLAGS",
	TagINSTALLCOLOR:                "INSTALLCOLOR",
	TagINSTALLTID:                  "INSTALLTID",
	TagREMOVETID:                   "REMOVETID",
	TagSHA1RHN:                     "SHA1RHN",
	TagRHNPLATFORM:                 "RHNPLATFORM",
	TagPLATFORM:                    "PLATFORM",
	TagPATCHESNAME:                 "PATCHESNAME",
	TagPATCHESFLAGS:                "PATCHESFLAGS",
	TagPATCHESVERSION:              "PATCHESVERSION",
	TagCACHECTIME:                  "CACHECTIME",
	TagCACHEPKGPATH:                "CACHEPKGPATH",
	TagCACHEPKGSIZE:                "CACHEPKGSIZE",
	TagCACHEPKGMTIME:               "CACHEPKGMTIME",
	TagFILECOLORS:                  "FILECOLORS",
	TagFILECLASS:                   "FILECLASS",
	TagCLASSDICT:                   "CLASSDICT",
	TagFILEDEPENDSX:                "FILEDEPENDSX",
	TagFILEDEPENDSN:                "FILEDEPENDSN",
	TagDEPENDSDICT:                 "DEPENDSDICT",
	TagSOURCEPKGID:                 "SOURCEPKGID",
	TagFILECONTEXTS:                "FILECONTEXTS",
	TagFSCONTEXTS:                  "FSCONTEXTS",
	TagRECONTEXTS:                  "RECONTEXTS",
	TagPOLICIES:                    "POLICIES",
	TagPRETRANS:                    "PRETRANS",
	TagPOSTTRANS:                   "POSTTRANS",
	TagPRETRANSPROG:                "PRETRANSPROG",
	TagPOSTTRANSPROG:               "POSTTRANSPROG",
	TagDISTTAG:                     "DISTTAG",
	TagOLDSUGGESTSNAME:             "OLDSUGGESTSNAME",
	TagOLDSUGGESTSVERSION:          "OLDSUGGESTSVERSION",
	TagOLDSUGGESTSFLAGS:            "OLDSUGGESTSFLAGS",
	TagOLDENHANCESNAME:             "OLDENHANCESNAME",
	TagOLDENHANCESVERSION:          "OLDENHANCESVERSION",
	TagOLDENHANCESFLAGS:            "OLDENHANCESFLAGS",
	TagPRIORITY:                    "PRIORITY",
	TagCVSID:                       "CVSID",
	TagBLINKPKGID:                  "BLINKPKGID",
	TagBLINKHDRID:                  "BLINKHDRID",
	TagBLINKNEVRA:                  "BLINKNEVRA",
	TagFLINKPKGID:                  "FLINKPKGID",
	TagFLINKHDRID:                  "FLINKHDRID",
	TagFLINKNEVRA:                  "FLINKNEVRA",
	TagPACKAGEORIGIN:               "PACKAGEORIGIN",
	TagTRIGGERPREIN:                "TRIGGERPREIN",
	TagBUILDSUGGESTS:               "BUILDSUGGESTS",
	TagBUILDENHANCES:               "BUILDENHANCES",
	TagSCRIPTSTATES:                "SCRIPTSTATES",
	TagSCRIPTMETRICS:               "SCRIPTMETRICS",
	TagBUILDCPUCLOCK:               "BUILDCPUCLOCK",
	TagFILEDIGESTALGOS:             "FILEDIGESTALGOS",
	TagVARIANTS:                    "VARIANTS",
	TagXMAJOR:                      "XMAJOR",
	TagXMINOR:                      "XMINOR",
	TagREPOTAG:                     "REPOTAG",
	TagKEYWORDS:                    "KEYWORDS",
	TagBUILDPLATFORMS:              "BUILDPLATFORMS",
	TagPACKAGECOLOR:                "PACKAGECOLOR",
	TagPACKAGEPREFCOLOR:            "PACKAGEPREFCOLOR",
	TagXATTRSDICT:                  "XATTRSDICT",
	TagFILEXATTRSX:                 "FILEXATTRSX",
	TagDEPATTRSDICT:                "DEPATTRSDICT",
	TagCONFLICTATTRSX:              "CONFLICTATTRSX",
	TagOBSOLETEATTRSX:              "OBSOLETEATTRSX",
	TagPROVIDEATTRSX:               "PROVIDEATTRSX",
	TagREQUIREATTRSX:               "REQUIREATTRSX",
	TagBUILDPROVIDES:               "BUILDPROVIDES",
	TagBUILDOBSOLETES:              "BUILDOBSOLETES",
	TagDBINSTANCE:                  "DBINSTANCE",
	TagNVRA:                        "NVRA",
	TagFILENAMES:                   "FILENAMES",
	TagFILEPROVIDE:                 "FILEPROVIDE",
	TagFILEREQUIRE:                 "FILEREQUIRE",
	TagFSNAMES:                     "FSNAMES",
	TagFSSIZES:                     "FSSIZES",
	TagTRIGGERCONDS:                "TRIGGERCONDS",
	TagTRIGGERTYPE:                 "TRIGGERTYPE",
	TagORIGFILENAMES:               "ORIGFILENAMES",
	TagLONGFILESIZES:               "LONGFILESIZES",
	TagLONGSIZE:                    "LONGSIZE",
	TagFILECAPS:                    "FILECAPS",
	TagFILEDIGESTALGO:              "FILEDIGESTALGO",
	TagBUGURL:                      "BUGURL",
	TagEVR:                         "EVR",
	TagNVR:                         "NVR",
	TagNEVR:                        "NEVR",
	TagNEVRA:                       "NEVRA",
	TagHEADERCOLOR:                 "HEADERCOLOR",
	TagVERBOSE:                     "VERBOSE",
	TagEPOCHNUM:                    "EPOCHNUM",
	TagPREINFLAGS:                  "PREINFLAGS",
	TagPOSTINFLAGS:                 "POSTINFLAGS",
	TagPREUNFLAGS:                  "PREUNFLAGS",
	TagPOSTUNFLAGS:                 "POSTUNFLAGS",
	TagPRETRANSFLAGS:               "PRETRANSFLAGS",
	TagPOSTTRANSFLAGS:              "POSTTRANSFLAGS",
	TagVERIFYSCRIPTFLAGS:           "VERIFYSCRIPTFLAGS",
	TagTRIGGERSCRIPTFLAGS:          "TRIGGERSCRIPTFLAGS",
	TagCOLLECTIONS:                 "COLLECTIONS",
	TagPOLICYNAMES:                 "POLICYNAMES",
	TagPOLICYTYPES:                 "POLICYTYPES",
	TagPOLICYTYPESINDEXES:          "POLICYTYPESINDEXES",
	TagPOLICYFLAGS:                 "POLICYFLAGS",
	TagVCS:                         "VCS",
	TagORDERNAME:                   "ORDERNAME",
	TagORDERVERSION:                "ORDERVERSION",
	TagORDERFLAGS:                  "ORDERFLAGS",
	TagMSSFMANIFEST:                "MSSFMANIFEST",
	TagMSSFDOMAIN:                  "MSSFDOMAIN",
	TagINSTFILENAMES:               "INSTFILENAMES",
	TagREQUIRENEVRS:                "REQUIRENEVRS",
	TagPROVIDENEVRS:                "PROVIDENEVRS",
	TagOBSOLETENEVRS:               "OBSOLETENEVRS",
	TagCONFLICTNEVRS:               "CONFLICTNEVRS",
	TagFILENLINKS:                  "FILENLINKS",
	TagRECOMMENDNAME:               "RECOMMENDNAME",
	TagRECOMMENDVERSION:            "RECOMMENDVERSION",
	TagRECOMMENDFLAGS:              "RECOMMENDFLAGS",
	TagSUGGESTNAME:                 "SUGGESTNAME",
	TagSUGGESTVERSION:              "SUGGESTVERSION",
	TagSUGGESTFLAGS:                "SUGGESTFLAGS",
	TagSUPPLEMENTNAME:              "SUPPLEMENTNAME",
	TagSUPPLEMENTVERSION:           "SUPPLEMENTVERSION",
	TagSUPPLEMENTFLAGS:             "SUPPLEMENTFLAGS",
	TagENHANCENAME:                 "ENHANCENAME",
	TagENHANCEVERSION:              "ENHANCEVERSION",
	TagENHANCEFLAGS:                "ENHANCEFLAGS",
	TagRECOMMENDNEVRS:              "RECOMMENDNEVRS",
	TagSUGGESTNEVRS:                "SUGGESTNEVRS",
	TagSUPPLEMENTNEVRS:             "SUPPLEMENTNEVRS",
	TagENHANCENEVRS:                "ENHANCENEVRS",
	TagENCODING:                    "ENCODING",
	TagFILETRIGGERIN:               "FILETRIGGERIN",
	TagFILETRIGGERUN:               "FILETRIGGERUN",
	TagFILETRIGGERPOSTUN:           "FILETRIGGERPOSTUN",
	TagFILETRIGGERSCRIPTS:          "FILETRIGGERSCRIPTS",
	TagFILETRIGGERSCRIPTPROG:       "FILETRIGGERSCRIPTPROG",
	TagFILETRIGGERSCRIPTFLAGS:      "FILETRIGGERSCRIPTFLAGS",
	TagFILETRIGGERNAME:             "FILETRIGGERNAME",
	TagFILETRIGGERINDEX:            "FILETRIGGERINDEX",
	TagFILETRIGGERVERSION:          "FILETRIGGERVERSION",
	TagFILETRIGGERFLAGS:            "FILETRIGGERFLAGS",
	TagTRANSFILETRIGGERIN:          "TRANSFILETRIGGERIN",
	TagTRANSFILETRIGGERUN:          "TRANSFILETRIGGERUN",
	TagTRANSFILETRIGGERPOSTUN:      "TRANSFILETRIGGERPOSTUN",
	TagTRANSFILETRIGGERSCRIPTS:     "TRANSFILETRIGGERSCRIPTS",
	TagTRANSFILETRIGGERSCRIPTPROG:  "TRANSFILETRIGGERSCRIPTPROG",
	TagTRANSFILETRIGGERSCRIPTFLAGS: "TRANSFILETRIGGERSCRIPTFLAGS",
	TagTRANSFILETRIGGERNAME:        "TRANSFILETRIGGERNAME",
	TagTRANSFILETRIGGERINDEX:       "TRANSFILETRIGGERINDEX",
	TagTRANSFILETRIGGERVERSION:     "TRANSFILETRIGGERVERSION",
	TagTRANSFILETRIGGERFLAGS:       "TRANSFILETRIGGERFLAGS",
	TagREMOVEPATHPOSTFIXES:         "REMOVEPATHPOSTFIXES",
	TagFILETRIGGERPRIORITIES:       "FILETRIGGERPRIORITIES",
	TagTRANSFILETRIGGERPRIORITIES:  "TRANSFILETRIGGERPRIORITIES",
	TagFILETRIGGERCONDS:            "FILETRIGGERCONDS",
	TagFILETRIGGERTYPE:             "FILETRIGGERTYPE",
	TagTRANSFILETRIGGERCONDS:       "TRANSFILETRIGGERCONDS",
	TagTRANSFILETRIGGERTYPE:        "TRANSFILETRIGGERTYPE",
	TagFILESIGNATURES:              "FILESIGNATURES",
	TagFILESIGNATURELENGTH:         "FILESIGNATURELENGTH",
	TagPAYLOADDIGEST:               "PAYLOADDIGEST",
	TagPAYLOADDIGESTALGO:           "PAYLOADDIGESTALGO",
	TagAUTOINSTALLED:               "AUTOINSTALLED",
	TagIDENTITY:                    "IDENTITY",
}
