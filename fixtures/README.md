# Example data

Real US Congress bill text from govinfo, committed byte for byte (no re-encoding, no pretty-printing) so every task builds and tests offline.
These are US government works in the public domain (17 U.S.C. 105). Content does not matter for development; coverage does.
Layout: `fixtures/bills/<bill-id>/status.xml` (BILLSTATUS), `fixtures/bills/<bill-id>/v/<version-code>/source.xml` (BILLS text version).
Size caps: at most 2 MB per bill and 20 MB in all ([design/tasks/README.md](../design/tasks/README.md#example-data-and-local-triggers)).

Retrieval date for every file: 2026-11-22. Exit-bill files and `status.xml` came one request at a time (2 s apart, descriptive user agent) from `scripts/fetch-bills.sh`; the corpus-only bills are members of the session's bulk archive `https://www.govinfo.gov/bulkdata/BILLS/119/1/hr/BILLS-119-1-hr.zip`, fetched once into the git-ignored `.local-data/` and scanned offline.
To verify: `sha256sum` each file and compare with the tables below (`test/acceptance/M1-04/run.sh` does).

## M1 exit bills

Chosen by M1-04 (see [design/tasks/m1/04-bill-xml-parser.md](../design/tasks/m1/04-bill-xml-parser.md)). Each version is at most about 30 KB of XML. None needs amendment XML.

| Bill id | Title | Versions | Covers |
| --- | --- | --- | --- |
| `us-119-hr-4313` | Hospital Inpatient Services Modernization Act | `ih`, `rh`, `eh`, `rfs` | amendatory text: quoted blocks, striking and inserting, US Code references; text changes between `ih` and `rh`; healthcare (pilot topic) |
| `us-119-hr-3062` | Promoting Cross-border Energy Infrastructure Act | `ih`, `rh`, `eh`, `rfs` | a repeal, striking and inserting, many US Code references |
| `us-119-hr-3427` | Water Resources Technical Assistance Review Act | `ih`, `rh`, `eh`, `rfs` | freestanding text (no quoted blocks or amendments) |

Still missing from the exit set: a table and a formula. None of the 8,042 documents of the 119th Congress 1st session House archive contains a `<formula>` element (formulas appear only as running text with `<fraction>`), and `<table>` occurs in 54 of them; the shortest are in the corpus below. See the M1-04 notes.

## Parser corpus (not exit bills)

| Bill id | Version | Source member | Covers |
| --- | --- | --- | --- |
| `us-119-hr-319` | `ih` | `BILLS-119hr319ih.xml` | a table (inside a quoted block): shortest bill with a `<table>` in the 119-1 House archive |
| `us-119-hr-1727` | `ih` | `BILLS-119hr1727ih.xml` | a second table |
| `us-119-hr-86` | `ih` | `BILLS-119hr86ih.xml` | a repeal, the shortest bill using `<continuation-text>` |
| `us-119-hr-3115` | `ih` | `BILLS-119hr3115ih.xml` | lists (`<list>`, `<list-item>`), fractions, long text (about 180 KB) |
| `us-119-hr-3944` | `rh` | `BILLS-119hr3944rh.xml` | appropriations structure (titles, `appropriations-*`, provisos), long bill (about 160 KB) |

## Files

| File | Bytes | Source URL | SHA-256 |
| --- | --- | --- | --- |
| `bills/us-119-hr-1727/v/ih/source.xml` | 5277 | https://www.govinfo.gov/bulkdata/BILLS/119/1/hr/BILLS-119-1-hr.zip (member BILLS-119hr1727ih.xml) | `6e1d577b568368d67443bf31363996fe4bd62b7f3f1241d6f182340db31b074f` |
| `bills/us-119-hr-3062/status.xml` | 36865 | https://www.govinfo.gov/bulkdata/BILLSTATUS/119/hr/BILLSTATUS-119hr3062.xml | `dc73918b87dd031f44d5a3ec621f609c583d054bab1e02d655262aca6dfb1a89` |
| `bills/us-119-hr-3062/v/eh/source.xml` | 17715 | https://www.govinfo.gov/content/pkg/BILLS-119hr3062eh/xml/BILLS-119hr3062eh.xml | `b0a0425f7bfc36ccc2880638ab2b070ec0b2438617dd43a11fe7b9dcffc4ad85` |
| `bills/us-119-hr-3062/v/ih/source.xml` | 17855 | https://www.govinfo.gov/content/pkg/BILLS-119hr3062ih/xml/BILLS-119hr3062ih.xml | `2fc9aea2d989797aeae2fac29c07622a788da9f1eacdb16d553ae4005eade0f7` |
| `bills/us-119-hr-3062/v/rfs/source.xml` | 17881 | https://www.govinfo.gov/content/pkg/BILLS-119hr3062rfs/xml/BILLS-119hr3062rfs.xml | `95b7024aa51fd89735a1ee2cad4d62ccef0b1e28407cb00bb0c5f6fd39f61105` |
| `bills/us-119-hr-3062/v/rh/source.xml` | 20246 | https://www.govinfo.gov/content/pkg/BILLS-119hr3062rh/xml/BILLS-119hr3062rh.xml | `11494d00e2e1cb9237f1f1c65b6a9cdb4aba454238c85bced7584837669fc67f` |
| `bills/us-119-hr-3115/v/ih/source.xml` | 181612 | https://www.govinfo.gov/bulkdata/BILLS/119/1/hr/BILLS-119-1-hr.zip (member BILLS-119hr3115ih.xml) | `db12d6b66cbd3fc4903a3eb2e7aa49b19eddc53f5286a5c99495b833fc286128` |
| `bills/us-119-hr-319/v/ih/source.xml` | 3344 | https://www.govinfo.gov/bulkdata/BILLS/119/1/hr/BILLS-119-1-hr.zip (member BILLS-119hr319ih.xml) | `7f0415ab7b81067638753cd1d3ad06f80102f7ee4b0da116793f95652ae5bc99` |
| `bills/us-119-hr-3427/status.xml` | 20247 | https://www.govinfo.gov/bulkdata/BILLSTATUS/119/hr/BILLSTATUS-119hr3427.xml | `8ba9636ecadc72c8e6236ebcc73d55488f7c413ac0c1a40d0ee5f034aa12bee0` |
| `bills/us-119-hr-3427/v/eh/source.xml` | 8809 | https://www.govinfo.gov/content/pkg/BILLS-119hr3427eh/xml/BILLS-119hr3427eh.xml | `e56cddc4d625cbe4a82ac83158d212dfb65b1a879cbe886ba19f6bea5ce71984` |
| `bills/us-119-hr-3427/v/ih/source.xml` | 8323 | https://www.govinfo.gov/content/pkg/BILLS-119hr3427ih/xml/BILLS-119hr3427ih.xml | `614d2e961ebe4d275c85dfe341d435410c6cb807e108b21a81eaceb48b407a4d` |
| `bills/us-119-hr-3427/v/rfs/source.xml` | 8979 | https://www.govinfo.gov/content/pkg/BILLS-119hr3427rfs/xml/BILLS-119hr3427rfs.xml | `75f322d757d40160e89fb7fe74bd873d28c9984333a3de2cd176abca93765e80` |
| `bills/us-119-hr-3427/v/rh/source.xml` | 10033 | https://www.govinfo.gov/content/pkg/BILLS-119hr3427rh/xml/BILLS-119hr3427rh.xml | `b6849a236435d6cb7aa15d386907711bdcb565c45627075d58130b9d01a4b994` |
| `bills/us-119-hr-3944/v/rh/source.xml` | 159959 | https://www.govinfo.gov/bulkdata/BILLS/119/1/hr/BILLS-119-1-hr.zip (member BILLS-119hr3944rh.xml) | `17a3cafba76c873d2d7471f76ca8e6d4d3479698613407e58455d9ef0e93e653` |
| `bills/us-119-hr-4313/status.xml` | 21903 | https://www.govinfo.gov/bulkdata/BILLSTATUS/119/hr/BILLSTATUS-119hr4313.xml | `a791a76f19511c8d1c42c320bb6e93ac3bfaa21768ffcfb9c2285b356b3f9133` |
| `bills/us-119-hr-4313/v/eh/source.xml` | 10605 | https://www.govinfo.gov/content/pkg/BILLS-119hr4313eh/xml/BILLS-119hr4313eh.xml | `862f1b0ff7548fc94977670d96293c6125082f60add977013843657c566e914a` |
| `bills/us-119-hr-4313/v/ih/source.xml` | 8409 | https://www.govinfo.gov/content/pkg/BILLS-119hr4313ih/xml/BILLS-119hr4313ih.xml | `9c7ff451173f693b498baf9a5a721e4f3e01ad98c56ccb7b8db1fa4d175bf6e8` |
| `bills/us-119-hr-4313/v/rfs/source.xml` | 10730 | https://www.govinfo.gov/content/pkg/BILLS-119hr4313rfs/xml/BILLS-119hr4313rfs.xml | `9699451c4f3ec6fc60b23e3f3b8fa8b7629ebe4d47ca1176130f3346d59c14f0` |
| `bills/us-119-hr-4313/v/rh/source.xml` | 11186 | https://www.govinfo.gov/content/pkg/BILLS-119hr4313rh/xml/BILLS-119hr4313rh.xml | `061e37a7e096552618da4c873e234dff1f83ad520e46f3dbaf4c7f2458abd8f4` |
| `bills/us-119-hr-86/v/ih/source.xml` | 2286 | https://www.govinfo.gov/bulkdata/BILLS/119/1/hr/BILLS-119-1-hr.zip (member BILLS-119hr86ih.xml) | `ec4ee4d12f23cfde6f5196972552e60ea04b4a9e19561d5e9375b45164a2bbbe` |

## US Code provisions (M1-07)

The sections the three M1 exit bills cite, cut byte for byte (the `<section>` element, nothing around it) from the Office of the Law Revision Counsel's XML title files at release point `119-142` (current through Public Law 119-142, November 10, 2026). US government works. Retrieval date: 2026-11-22; the zips came one request at a time (3 s apart, descriptive user agent) from `https://uscode.house.gov/download/releasepoints/us/pl/119/142/xml_usc<TT>@119-142.zip` into the git-ignored `.local-data/uscode/119-142/`; `go run ./scripts/extract-uscode-fixtures` cut the sections. Layout: `fixtures/uscode/119-142/t<title>-s<section>.xml`, with the section's en dash written as a hyphen in the file name. The release date is not in the files; it is the release point's (2026-11-10). Tests wrap a fragment in a `<uscDoc identifier="/us/usc/t<title>">` root before parsing.

| Title | Zip SHA-256 (`xml_usc<TT>@119-142.zip`) | SHA-256 of `usc<TT>.xml` inside it |
| --- | --- | --- |
| 3 | `840e4360cd138663246a8a934c824b9b81ded142533e1adc663bc1cbe119d9bd` | `c8f5c97c578aacfa66f3c7517d54376b1d4e9fb7f5b5f3563a7941ed930c11b6` |
| 15 | `0de372cac2ecdfd08230e4017968227f62358f2c9a3a20a2b3de77614118b7f5` | `36294c32ef49661c4f115d1608490983d33d3403aa0dc2bdb1c7855ed21ffabc` |
| 16 | `6b824d53508fa103706c1a6cd231ac023dbee150d51875492c93cf95a70ae1bc` | `c0732c3b6cb2abb23722deb1cd8441ccb3c6f3e7368ae3290e27cb87eb1f0136` |
| 42 | `25738c82e501e6844f7744c492f091500da1497dd3e2074a7882c889c2b14de6` | `d514b50fcfb7a60cb12d0ce5e55263f91d52a6a30840cf62000c67d4cb17c9f5` |

| File | Bytes | Source (title file at release point 119-142) | SHA-256 of the file |
| --- | --- | --- | --- |
| `uscode/119-142/t15-s717a.xml` | 6859 | https://uscode.house.gov/download/releasepoints/us/pl/119/142/xml_usc15@119-142.zip | `82e0e9425308c8cf79f6d51f854528bec79238652fed4fc500dbe2ca2b233c17` |
| `uscode/119-142/t15-s717b.xml` | 22040 | https://uscode.house.gov/download/releasepoints/us/pl/119/142/xml_usc15@119-142.zip | `e7e19b862c42274e5f171f019fbd97d26459b3d14a924434e4569bb526ff8f3a` |
| `uscode/119-142/t16-s796.xml` | 33672 | https://uscode.house.gov/download/releasepoints/us/pl/119/142/xml_usc16@119-142.zip | `b78c8f499469be07206c15e1019e12fcdfbaef501678a006c8ffa009d4904807` |
| `uscode/119-142/t16-s824a-4.xml` | 9955 | https://uscode.house.gov/download/releasepoints/us/pl/119/142/xml_usc16@119-142.zip | `7e24ed7646cde696494a7ba1cb7894270a24c6731f52ae7db3769be70dbae95e` |
| `uscode/119-142/t16-s824a.xml` | 19439 | https://uscode.house.gov/download/releasepoints/us/pl/119/142/xml_usc16@119-142.zip | `1c3fa3b0d0e10d722d6820a42ba70cef33f4f31c0407b16088b442d10785647f` |
| `uscode/119-142/t16-s824o.xml` | 29908 | https://uscode.house.gov/download/releasepoints/us/pl/119/142/xml_usc16@119-142.zip | `995379e8c74a1064b0156edc5f3318af4f10e6eac81b4c26d1b9509771707577` |
| `uscode/119-142/t3-s301.xml` | 200391 | https://uscode.house.gov/download/releasepoints/us/pl/119/142/xml_usc03@119-142.zip | `9f81af7e1e0802e9904e3b6980f9c990538f38cdc711c6ae5710c8fa4bc760a1` |
| `uscode/119-142/t42-s1395cc-7.xml` | 26124 | https://uscode.house.gov/download/releasepoints/us/pl/119/142/xml_usc42@119-142.zip | `f8bd5ea46e67ae108181eeb3b7645afa273b73fa86a21378bd070e3880749c0c` |
| `uscode/119-142/t42-s1395iii.xml` | 24124 | https://uscode.house.gov/download/releasepoints/us/pl/119/142/xml_usc42@119-142.zip | `4d192031e9ff981c43a00f73360e02f2c28c722bcbe3aad8edbb5ce9913be7af` |
| `uscode/119-142/t42-s4321.xml` | 245359 | https://uscode.house.gov/download/releasepoints/us/pl/119/142/xml_usc42@119-142.zip | `a0da1766c9bfe46e94ff4b9fb973058ca034249876690e231d233a23d43072c1` |
| `uscode/119-142/t42-s7151.xml` | 18668 | https://uscode.house.gov/download/releasepoints/us/pl/119/142/xml_usc42@119-142.zip | `076e7ab41746de3befa6ea75f8544140784c9ae6d40f254c2adff85e2def2e97` |
