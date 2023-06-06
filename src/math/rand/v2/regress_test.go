// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Test that random number sequences generated by a specific seed
// do not change from version to version.
//
// Do NOT make changes to the golden outputs. If bugs need to be fixed
// in the underlying code, find ways to fix them that do not affect the
// outputs.

package rand_test

import (
	"bytes"
	"flag"
	"fmt"
	"go/format"
	"io"
	. "math/rand/v2"
	"os"
	"reflect"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "update golden results for regression test")

func TestRegress(t *testing.T) {
	var int32s = []int32{1, 10, 32, 1 << 20, 1<<20 + 1, 1000000000, 1 << 30, 1<<31 - 2, 1<<31 - 1}
	var uint32s = []uint32{1, 10, 32, 1 << 20, 1<<20 + 1, 1000000000, 1 << 30, 1<<31 - 2, 1<<31 - 1, 1<<32 - 2, 1<<32 - 1}
	var int64s = []int64{1, 10, 32, 1 << 20, 1<<20 + 1, 1000000000, 1 << 30, 1<<31 - 2, 1<<31 - 1, 1000000000000000000, 1 << 60, 1<<63 - 2, 1<<63 - 1}
	var uint64s = []uint64{1, 10, 32, 1 << 20, 1<<20 + 1, 1000000000, 1 << 30, 1<<31 - 2, 1<<31 - 1, 1000000000000000000, 1 << 60, 1<<63 - 2, 1<<63 - 1, 1<<64 - 2, 1<<64 - 1}
	var permSizes = []int{0, 1, 5, 8, 9, 10, 16}

	n := reflect.TypeOf(New(NewSource(1))).NumMethod()
	p := 0
	var buf bytes.Buffer
	if *update {
		fmt.Fprintf(&buf, "var regressGolden = []any{\n")
	}
	for i := 0; i < n; i++ {
		if *update && i > 0 {
			fmt.Fprintf(&buf, "\n")
		}
		r := New(NewSource(1))
		rv := reflect.ValueOf(r)
		m := rv.Type().Method(i)
		mv := rv.Method(i)
		mt := mv.Type()
		if mt.NumOut() == 0 {
			continue
		}
		for repeat := 0; repeat < 20; repeat++ {
			var args []reflect.Value
			var argstr string
			if mt.NumIn() == 1 {
				var x any
				switch mt.In(0).Kind() {
				default:
					t.Fatalf("unexpected argument type for r.%s", m.Name)

				case reflect.Int:
					if m.Name == "Perm" {
						x = permSizes[repeat%len(permSizes)]
						break
					}
					big := int64s[repeat%len(int64s)]
					if int64(int(big)) != big {
						// On 32-bit machine.
						// Consume an Int64 like on a 64-bit machine,
						// to keep the golden data the same on different architectures.
						r.Int64N(big)
						if *update {
							t.Fatalf("must run -update on 64-bit machine")
						}
						p++
						continue
					}
					x = int(big)

				case reflect.Uint:
					big := uint64s[repeat%len(uint64s)]
					if uint64(uint(big)) != big {
						r.Uint64N(big) // what would happen on 64-bit machine, to keep stream in sync
						if *update {
							t.Fatalf("must run -update on 64-bit machine")
						}
						p++
						continue
					}
					x = uint(big)

				case reflect.Int32:
					x = int32s[repeat%len(int32s)]

				case reflect.Int64:
					x = int64s[repeat%len(int64s)]

				case reflect.Uint32:
					x = uint32s[repeat%len(uint32s)]

				case reflect.Uint64:
					x = uint64s[repeat%len(uint64s)]
				}
				argstr = fmt.Sprint(x)
				args = append(args, reflect.ValueOf(x))
			}

			var out any
			out = mv.Call(args)[0].Interface()
			if m.Name == "Int" || m.Name == "IntN" {
				out = int64(out.(int))
			}
			if m.Name == "Uint" || m.Name == "UintN" {
				out = uint64(out.(uint))
			}
			if *update {
				var val string
				big := int64(1 << 60)
				if int64(int(big)) != big && (m.Name == "Int" || m.Name == "IntN") {
					// 32-bit machine cannot print 64-bit results
					val = "truncated"
				} else if reflect.TypeOf(out).Kind() == reflect.Slice {
					val = fmt.Sprintf("%#v", out)
				} else {
					val = fmt.Sprintf("%T(%v)", out, out)
				}
				fmt.Fprintf(&buf, "\t%s, // %s(%s)\n", val, m.Name, argstr)
			} else if p >= len(regressGolden) {
				t.Errorf("r.%s(%s) = %v, missing golden value", m.Name, argstr, out)
			} else {
				want := regressGolden[p]
				if m.Name == "Int" {
					want = int64(int(uint(want.(int64)) << 1 >> 1))
				}
				if !reflect.DeepEqual(out, want) {
					t.Errorf("r.%s(%s) = %v, want %v", m.Name, argstr, out, want)
				}
			}
			p++
		}
	}
	if *update {
		replace(t, "regress_test.go", buf.Bytes())
	}
}

func TestUpdateExample(t *testing.T) {
	if !*update {
		t.Skip("-update not given")
	}

	oldStdout := os.Stdout
	defer func() {
		os.Stdout = oldStdout
	}()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()

	go func() {
		os.Stdout = w
		Example_rand()
		os.Stdout = oldStdout
		w.Close()
	}()
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	fmt.Fprintf(&buf, "\t// Output:\n")
	for _, line := range strings.Split(string(out), "\n") {
		if line != "" {
			fmt.Fprintf(&buf, "\t// %s\n", line)
		}
	}

	replace(t, "example_test.go", buf.Bytes())

	// Exit so that Example_rand cannot fail.
	fmt.Printf("UPDATED; ignore non-zero exit status\n")
	os.Exit(1)
}

// replace substitutes the definition text from new into the content of file.
// The text in new is of the form
//
//	var whatever = T{
//		...
//	}
//
// Replace searches file for an exact match for the text of the first line,
// finds the closing brace, and then substitutes new for what used to be in the file.
// This lets us update the regressGolden table during go test -update.
func replace(t *testing.T, file string, new []byte) {
	first, _, _ := bytes.Cut(new, []byte("\n"))
	first = append(append([]byte("\n"), first...), '\n')
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	i := bytes.Index(data, first)
	if i < 0 {
		t.Fatalf("cannot find %q in %s", first, file)
	}
	j := bytes.Index(data[i+1:], []byte("\n}\n"))
	if j < 0 {
		t.Fatalf("cannot find end in %s", file)
	}
	data = append(append(data[:i+1:i+1], new...), data[i+1+j+1:]...)
	data, err = format.Source(data)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, data, 0666); err != nil {
		t.Fatal(err)
	}
}

var regressGolden = []any{
	float64(0.1835616265352068),  // ExpFloat64()
	float64(0.1747899228736829),  // ExpFloat64()
	float64(2.369801563222863),   // ExpFloat64()
	float64(1.8580757676846802),  // ExpFloat64()
	float64(0.35731123690292155), // ExpFloat64()
	float64(0.5998175837039783),  // ExpFloat64()
	float64(0.466149534807967),   // ExpFloat64()
	float64(1.333748223451787),   // ExpFloat64()
	float64(0.05019983258513916), // ExpFloat64()
	float64(1.4143832256421573),  // ExpFloat64()
	float64(0.7274094466687158),  // ExpFloat64()
	float64(0.9595398235158843),  // ExpFloat64()
	float64(1.3010086894917756),  // ExpFloat64()
	float64(0.8678483737499929),  // ExpFloat64()
	float64(0.7958895614497015),  // ExpFloat64()
	float64(0.12235329704897674), // ExpFloat64()
	float64(1.1625413819613253),  // ExpFloat64()
	float64(1.2603945934386542),  // ExpFloat64()
	float64(0.22199446394172706), // ExpFloat64()
	float64(2.248962105270165),   // ExpFloat64()

	float32(0.6046603),  // Float32()
	float32(0.9405091),  // Float32()
	float32(0.6645601),  // Float32()
	float32(0.4377142),  // Float32()
	float32(0.4246375),  // Float32()
	float32(0.68682307), // Float32()
	float32(0.06563702), // Float32()
	float32(0.15651925), // Float32()
	float32(0.09696952), // Float32()
	float32(0.30091187), // Float32()
	float32(0.51521266), // Float32()
	float32(0.81363994), // Float32()
	float32(0.21426387), // Float32()
	float32(0.3806572),  // Float32()
	float32(0.31805816), // Float32()
	float32(0.46888983), // Float32()
	float32(0.28303415), // Float32()
	float32(0.29310185), // Float32()
	float32(0.67908466), // Float32()
	float32(0.21855305), // Float32()

	float64(0.6046602879796196),  // Float64()
	float64(0.9405090880450124),  // Float64()
	float64(0.6645600532184904),  // Float64()
	float64(0.4377141871869802),  // Float64()
	float64(0.4246374970712657),  // Float64()
	float64(0.6868230728671094),  // Float64()
	float64(0.06563701921747622), // Float64()
	float64(0.15651925473279124), // Float64()
	float64(0.09696951891448456), // Float64()
	float64(0.30091186058528707), // Float64()
	float64(0.5152126285020654),  // Float64()
	float64(0.8136399609900968),  // Float64()
	float64(0.21426387258237492), // Float64()
	float64(0.380657189299686),   // Float64()
	float64(0.31805817433032985), // Float64()
	float64(0.4688898449024232),  // Float64()
	float64(0.28303415118044517), // Float64()
	float64(0.29310185733681576), // Float64()
	float64(0.6790846759202163),  // Float64()
	float64(0.21855305259276428), // Float64()

	int64(5577006791947779410), // Int()
	int64(8674665223082153551), // Int()
	int64(6129484611666145821), // Int()
	int64(4037200794235010051), // Int()
	int64(3916589616287113937), // Int()
	int64(6334824724549167320), // Int()
	int64(605394647632969758),  // Int()
	int64(1443635317331776148), // Int()
	int64(894385949183117216),  // Int()
	int64(2775422040480279449), // Int()
	int64(4751997750760398084), // Int()
	int64(7504504064263669287), // Int()
	int64(1976235410884491574), // Int()
	int64(3510942875414458836), // Int()
	int64(2933568871211445515), // Int()
	int64(4324745483838182873), // Int()
	int64(2610529275472644968), // Int()
	int64(2703387474910584091), // Int()
	int64(6263450610539110790), // Int()
	int64(2015796113853353331), // Int()

	int32(649249040),  // Int32()
	int32(1009863943), // Int32()
	int32(1787307747), // Int32()
	int32(1543733853), // Int32()
	int32(455951040),  // Int32()
	int32(737470659),  // Int32()
	int32(1144219036), // Int32()
	int32(1241803094), // Int32()
	int32(104120228),  // Int32()
	int32(1396843474), // Int32()
	int32(553205347),  // Int32()
	int32(873639255),  // Int32()
	int32(1303805905), // Int32()
	int32(408727544),  // Int32()
	int32(1415254188), // Int32()
	int32(503466637),  // Int32()
	int32(1377647429), // Int32()
	int32(1388457546), // Int32()
	int32(729161618),  // Int32()
	int32(1308411377), // Int32()

	int32(0),          // Int32N(1)
	int32(4),          // Int32N(10)
	int32(29),         // Int32N(32)
	int32(883715),     // Int32N(1048576)
	int32(222632),     // Int32N(1048577)
	int32(343411536),  // Int32N(1000000000)
	int32(957743134),  // Int32N(1073741824)
	int32(1241803092), // Int32N(2147483646)
	int32(104120228),  // Int32N(2147483647)
	int32(0),          // Int32N(1)
	int32(2),          // Int32N(10)
	int32(7),          // Int32N(32)
	int32(96566),      // Int32N(1048576)
	int32(199574),     // Int32N(1048577)
	int32(659029087),  // Int32N(1000000000)
	int32(606492121),  // Int32N(1073741824)
	int32(1377647428), // Int32N(2147483646)
	int32(1388457546), // Int32N(2147483647)
	int32(0),          // Int32N(1)
	int32(6),          // Int32N(10)

	int64(5577006791947779410), // Int64()
	int64(8674665223082153551), // Int64()
	int64(6129484611666145821), // Int64()
	int64(4037200794235010051), // Int64()
	int64(3916589616287113937), // Int64()
	int64(6334824724549167320), // Int64()
	int64(605394647632969758),  // Int64()
	int64(1443635317331776148), // Int64()
	int64(894385949183117216),  // Int64()
	int64(2775422040480279449), // Int64()
	int64(4751997750760398084), // Int64()
	int64(7504504064263669287), // Int64()
	int64(1976235410884491574), // Int64()
	int64(3510942875414458836), // Int64()
	int64(2933568871211445515), // Int64()
	int64(4324745483838182873), // Int64()
	int64(2610529275472644968), // Int64()
	int64(2703387474910584091), // Int64()
	int64(6263450610539110790), // Int64()
	int64(2015796113853353331), // Int64()

	int64(0),                   // Int64N(1)
	int64(4),                   // Int64N(10)
	int64(29),                  // Int64N(32)
	int64(883715),              // Int64N(1048576)
	int64(222632),              // Int64N(1048577)
	int64(343411536),           // Int64N(1000000000)
	int64(957743134),           // Int64N(1073741824)
	int64(1241803092),          // Int64N(2147483646)
	int64(104120228),           // Int64N(2147483647)
	int64(650455930292643530),  // Int64N(1000000000000000000)
	int64(140311732333010180),  // Int64N(1152921504606846976)
	int64(3752252032131834642), // Int64N(9223372036854775806)
	int64(5599803723869633690), // Int64N(9223372036854775807)
	int64(0),                   // Int64N(1)
	int64(6),                   // Int64N(10)
	int64(25),                  // Int64N(32)
	int64(920424),              // Int64N(1048576)
	int64(677958),              // Int64N(1048577)
	int64(339542337),           // Int64N(1000000000)
	int64(701992307),           // Int64N(1073741824)

	int64(0),                   // IntN(1)
	int64(4),                   // IntN(10)
	int64(29),                  // IntN(32)
	int64(883715),              // IntN(1048576)
	int64(222632),              // IntN(1048577)
	int64(343411536),           // IntN(1000000000)
	int64(957743134),           // IntN(1073741824)
	int64(1241803092),          // IntN(2147483646)
	int64(104120228),           // IntN(2147483647)
	int64(650455930292643530),  // IntN(1000000000000000000)
	int64(140311732333010180),  // IntN(1152921504606846976)
	int64(3752252032131834642), // IntN(9223372036854775806)
	int64(5599803723869633690), // IntN(9223372036854775807)
	int64(0),                   // IntN(1)
	int64(6),                   // IntN(10)
	int64(25),                  // IntN(32)
	int64(920424),              // IntN(1048576)
	int64(677958),              // IntN(1048577)
	int64(339542337),           // IntN(1000000000)
	int64(701992307),           // IntN(1073741824)

	float64(0.6694336828657225),  // NormFloat64()
	float64(0.7506128421991493),  // NormFloat64()
	float64(-0.5466367925077582), // NormFloat64()
	float64(-0.8240444698703802), // NormFloat64()
	float64(0.11563765115029284), // NormFloat64()
	float64(-1.3442355710948637), // NormFloat64()
	float64(-1.0654999977586854), // NormFloat64()
	float64(0.15938628997241455), // NormFloat64()
	float64(-0.8046314635002316), // NormFloat64()
	float64(0.8323920113630076),  // NormFloat64()
	float64(1.0611019472659846),  // NormFloat64()
	float64(-0.8814992544664111), // NormFloat64()
	float64(0.9236344788106081),  // NormFloat64()
	float64(-1.2854378982224413), // NormFloat64()
	float64(0.4683572952232405),  // NormFloat64()
	float64(-0.5065217527091702), // NormFloat64()
	float64(-0.6460803205194869), // NormFloat64()
	float64(0.7913615856789362),  // NormFloat64()
	float64(-1.6119549224461807), // NormFloat64()
	float64(0.16216183438701695), // NormFloat64()

	[]int{},                             // Perm(0)
	[]int{0},                            // Perm(1)
	[]int{0, 4, 2, 1, 3},                // Perm(5)
	[]int{2, 4, 5, 0, 7, 1, 3, 6},       // Perm(8)
	[]int{6, 4, 1, 5, 7, 3, 0, 8, 2},    // Perm(9)
	[]int{8, 0, 1, 2, 3, 9, 5, 4, 7, 6}, // Perm(10)
	[]int{0, 13, 14, 7, 1, 4, 15, 10, 11, 12, 9, 5, 3, 6, 8, 2}, // Perm(16)
	[]int{},                             // Perm(0)
	[]int{0},                            // Perm(1)
	[]int{3, 2, 4, 0, 1},                // Perm(5)
	[]int{7, 1, 6, 4, 2, 3, 5, 0},       // Perm(8)
	[]int{1, 7, 2, 6, 3, 5, 8, 4, 0},    // Perm(9)
	[]int{1, 5, 7, 0, 3, 6, 4, 9, 2, 8}, // Perm(10)
	[]int{6, 13, 2, 11, 14, 7, 10, 12, 4, 5, 3, 0, 15, 9, 1, 8}, // Perm(16)
	[]int{},                             // Perm(0)
	[]int{0},                            // Perm(1)
	[]int{0, 4, 2, 1, 3},                // Perm(5)
	[]int{0, 7, 1, 4, 3, 6, 2, 5},       // Perm(8)
	[]int{1, 3, 0, 4, 5, 2, 8, 7, 6},    // Perm(9)
	[]int{5, 4, 7, 9, 6, 1, 0, 3, 8, 2}, // Perm(10)

	uint32(1298498081), // Uint32()
	uint32(2019727887), // Uint32()
	uint32(3574615495), // Uint32()
	uint32(3087467707), // Uint32()
	uint32(911902081),  // Uint32()
	uint32(1474941318), // Uint32()
	uint32(2288438073), // Uint32()
	uint32(2483606188), // Uint32()
	uint32(208240456),  // Uint32()
	uint32(2793686948), // Uint32()
	uint32(1106410694), // Uint32()
	uint32(1747278511), // Uint32()
	uint32(2607611810), // Uint32()
	uint32(817455089),  // Uint32()
	uint32(2830508376), // Uint32()
	uint32(1006933274), // Uint32()
	uint32(2755294859), // Uint32()
	uint32(2776915093), // Uint32()
	uint32(1458323237), // Uint32()
	uint32(2616822754), // Uint32()

	uint32(0),          // Uint32N(1)
	uint32(4),          // Uint32N(10)
	uint32(29),         // Uint32N(32)
	uint32(883715),     // Uint32N(1048576)
	uint32(222632),     // Uint32N(1048577)
	uint32(343411536),  // Uint32N(1000000000)
	uint32(957743134),  // Uint32N(1073741824)
	uint32(1241803092), // Uint32N(2147483646)
	uint32(104120228),  // Uint32N(2147483647)
	uint32(2793686946), // Uint32N(4294967294)
	uint32(1106410694), // Uint32N(4294967295)
	uint32(0),          // Uint32N(1)
	uint32(6),          // Uint32N(10)
	uint32(20),         // Uint32N(32)
	uint32(240907),     // Uint32N(1048576)
	uint32(245833),     // Uint32N(1048577)
	uint32(641517075),  // Uint32N(1000000000)
	uint32(340335899),  // Uint32N(1073741824)
	uint32(729161617),  // Uint32N(2147483646)
	uint32(1308411376), // Uint32N(2147483647)

	uint64(5577006791947779410),  // Uint64()
	uint64(8674665223082153551),  // Uint64()
	uint64(15352856648520921629), // Uint64()
	uint64(13260572831089785859), // Uint64()
	uint64(3916589616287113937),  // Uint64()
	uint64(6334824724549167320),  // Uint64()
	uint64(9828766684487745566),  // Uint64()
	uint64(10667007354186551956), // Uint64()
	uint64(894385949183117216),   // Uint64()
	uint64(11998794077335055257), // Uint64()
	uint64(4751997750760398084),  // Uint64()
	uint64(7504504064263669287),  // Uint64()
	uint64(11199607447739267382), // Uint64()
	uint64(3510942875414458836),  // Uint64()
	uint64(12156940908066221323), // Uint64()
	uint64(4324745483838182873),  // Uint64()
	uint64(11833901312327420776), // Uint64()
	uint64(11926759511765359899), // Uint64()
	uint64(6263450610539110790),  // Uint64()
	uint64(11239168150708129139), // Uint64()

	uint64(0),                    // Uint64N(1)
	uint64(4),                    // Uint64N(10)
	uint64(29),                   // Uint64N(32)
	uint64(883715),               // Uint64N(1048576)
	uint64(222632),               // Uint64N(1048577)
	uint64(343411536),            // Uint64N(1000000000)
	uint64(957743134),            // Uint64N(1073741824)
	uint64(1241803092),           // Uint64N(2147483646)
	uint64(104120228),            // Uint64N(2147483647)
	uint64(650455930292643530),   // Uint64N(1000000000000000000)
	uint64(140311732333010180),   // Uint64N(1152921504606846976)
	uint64(3752252032131834642),  // Uint64N(9223372036854775806)
	uint64(5599803723869633690),  // Uint64N(9223372036854775807)
	uint64(3510942875414458835),  // Uint64N(18446744073709551614)
	uint64(12156940908066221322), // Uint64N(18446744073709551615)
	uint64(0),                    // Uint64N(1)
	uint64(6),                    // Uint64N(10)
	uint64(27),                   // Uint64N(32)
	uint64(205190),               // Uint64N(1048576)
	uint64(638873),               // Uint64N(1048577)

	uint64(0),                    // UintN(1)
	uint64(4),                    // UintN(10)
	uint64(29),                   // UintN(32)
	uint64(883715),               // UintN(1048576)
	uint64(222632),               // UintN(1048577)
	uint64(343411536),            // UintN(1000000000)
	uint64(957743134),            // UintN(1073741824)
	uint64(1241803092),           // UintN(2147483646)
	uint64(104120228),            // UintN(2147483647)
	uint64(650455930292643530),   // UintN(1000000000000000000)
	uint64(140311732333010180),   // UintN(1152921504606846976)
	uint64(3752252032131834642),  // UintN(9223372036854775806)
	uint64(5599803723869633690),  // UintN(9223372036854775807)
	uint64(3510942875414458835),  // UintN(18446744073709551614)
	uint64(12156940908066221322), // UintN(18446744073709551615)
	uint64(0),                    // UintN(1)
	uint64(6),                    // UintN(10)
	uint64(27),                   // UintN(32)
	uint64(205190),               // UintN(1048576)
	uint64(638873),               // UintN(1048577)
}
