package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"

	flag "github.com/ogier/pflag"
)

var _ = fmt.Println // for debugging

// cli flags
var (
	autoskip   = flag.BoolP("autoskip", "a", false, "toggle autoskip (* replaces nul lines")
	binary     = flag.BoolP("binary", "b", false, "binary dump, incompatible with -ps, -i, -r")
	columns    = flag.IntP("cols", "c", -1, "format <cols> octets per line")
	ebcdic     = flag.BoolP("ebcdic", "E", false, "use EBCDIC instead of ASCII")
	group      = flag.IntP("group", "g", -1, "num of octets per group")
	cfmt       = flag.BoolP("include", "i", false, "output in C include format")
	length     = flag.Int64P("len", "l", -1, "stop after len octets")
	postscript = flag.Bool("ps", false, "output in postscript plain hd style")
	reverse    = flag.BoolP("reverse", "r", false, "convert hex to binary")
	offset     = flag.Int("off", 0, "revert with offset")
	seek       = flag.IntP("seek", "s", 0, "start at seek bytes abs")
	upper      = flag.BoolP("uppercase", "u", false, "use uppercase hex letters")
	version    = flag.BoolP("version", "v", false, "print version")
)

// constants used in xxd()
const (
	ebcdicOffset = 0x40
)

// variables used in xxd()
var (
	space        = []byte(" ")
	doubleSpace  = []byte("  ")
	dot          = []byte(".")
	newLine      = []byte("\n")
	zeroHeader   = []byte("0000000: ")
	unsignedChar = []byte("unsigned char ")
	brackets     = []byte("[] = {")
	asterisk     = []byte("*")
	hexPrefix    = []byte("0x")
	comma        = []byte(",")
)

// ascii -> ebcdic lookup table
var ebcdicTable = []byte{
	0040, 0240, 0241, 0242, 0243, 0244, 0245, 0246,
	0247, 0250, 0325, 0056, 0074, 0050, 0053, 0174,
	0046, 0251, 0252, 0253, 0254, 0255, 0256, 0257,
	0260, 0261, 0041, 0044, 0052, 0051, 0073, 0176,
	0055, 0057, 0262, 0263, 0264, 0265, 0266, 0267,
	0270, 0271, 0313, 0054, 0045, 0137, 0076, 0077,
	0272, 0273, 0274, 0275, 0276, 0277, 0300, 0301,
	0302, 0140, 0072, 0043, 0100, 0047, 0075, 0042,
	0303, 0141, 0142, 0143, 0144, 0145, 0146, 0147,
	0150, 0151, 0304, 0305, 0306, 0307, 0310, 0311,
	0312, 0152, 0153, 0154, 0155, 0156, 0157, 0160,
	0161, 0162, 0136, 0314, 0315, 0316, 0317, 0320,
	0321, 0345, 0163, 0164, 0165, 0166, 0167, 0170,
	0171, 0172, 0322, 0323, 0324, 0133, 0326, 0327,
	0330, 0331, 0332, 0333, 0334, 0335, 0336, 0337,
	0340, 0341, 0342, 0343, 0344, 0135, 0346, 0347,
	0173, 0101, 0102, 0103, 0104, 0105, 0106, 0107,
	0110, 0111, 0350, 0351, 0352, 0353, 0354, 0355,
	0175, 0112, 0113, 0114, 0115, 0116, 0117, 0120,
	0121, 0122, 0356, 0357, 0360, 0361, 0362, 0363,
	0134, 0237, 0123, 0124, 0125, 0126, 0127, 0130,
	0131, 0132, 0364, 0365, 0366, 0367, 0370, 0371,
	0060, 0061, 0062, 0063, 0064, 0065, 0066, 0067,
	0070, 0071, 0372, 0373, 0374, 0375, 0376, 0377,
}

// hex lookup table for hexEncode()
const (
	ldigits = "0123456789abcdef"
	udigits = "0123456789ABCDEF"
)

func hexEncode(dst, src []byte, hextable string) {
	for i, v := range src {
		dst[i*2] = hextable[v>>4]
		dst[i*2+1] = hextable[v&0x0f]
	}
}

func binaryEncode(dst, src []byte) {
	d := uint(0)
	for i := 7; i >= 0; i-- {
		if src[0]&(1<<d) == 0 {
			dst[i] = '0'
		} else {
			dst[i] = '1'
		}
		d++
	}
}

/*func cFmtEncode(dst, src []byte) {
	for i := 0; i < len(src); i++ {
		dst[i] = hexEncode(dst, src, hextable)
	}
}*/

func empty(b *[]byte) bool {
	for _, v := range *b {
		if v != 0 {
			return false
		}
	}
	return true
}

func xxd(r io.Reader, fname string) error {
	// Define our writer inside xxd() so we can periodically flush the buffer
	// TO-DO: This could be moved out again?
	w := bufio.NewWriter(os.Stdout)

	var (
		lineOffset int64
		hexOffset  = make([]byte, 6)
		caps       = ldigits
		cols       int
		octs       int
		doCHeader  = true
		cVariable  = make([]byte, len(fname))
		nulLine    int64
		totalOcts  int64
	)

	// Generate C variable in filename_format format
	// e.g. foo.txt -> foo_txt
	if *cfmt {
		i := 0
		for i < len(fname) {
			if fname[i] != '.' {
				cVariable[i] = fname[i]
			} else {
				cVariable[i] = '_'
			}
			i++
		}
	}

	// Switch between upper- and lower-case hex chars
	if *upper {
		caps = udigits
	}

	// xxd -bpi FILE outputs in binary format
	// xxd -b -p -i FILE outputs in C format
	// simply catch the last option since that's what I assume the author
	// wanted...
	if *columns == -1 {
		switch true {
		case *postscript:
			cols = 30
		case *cfmt:
			cols = 12
		case *binary:
			cols = 6
		default:
			cols = 16
		}
	} else {
		cols = *columns
	}

	// See above comment
	if *group == -1 {
		switch true {
		case *binary:
			octs = 8
		case *postscript, *cfmt:
			octs = 0
		default:
			octs = 2
		}
	} else {
		octs = *group
	}

	if *length != -1 {
		if *length < int64(cols) {
			cols = int(*length)
		}
	}

	if octs < 1 {
		octs = cols
	}

	// These are bumped down from the beginning of the function in order to
	// allow for their sizes to be allocated based on the user's speficiations
	var (
		line = make([]byte, cols)
		char = make([]byte, octs)
		odd  = *postscript && *cfmt
	)

	r = bufio.NewReader(r)
	for {
		n, err := io.ReadFull(r, line)
		if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
			return err
		}
		if n == 0 {
			return nil
		}

		if *length != -1 {
			if totalOcts == *length {
				break
			}
			totalOcts += *length
		}

		if *autoskip && empty(&line) {
			if nulLine == 1 {
				w.Write(asterisk)
				w.Write(newLine)
			}

			nulLine++

			if nulLine > 1 {
				lineOffset++ // continue to incriment our offset
				continue
			}
		}

		if *binary || !odd {
			// Line offset
			hexOffset = strconv.AppendInt(hexOffset[0:0], lineOffset, 16)
			w.Write(zeroHeader[0:(6 - len(hexOffset))])
			w.Write(hexOffset)
			w.Write(zeroHeader[6:])
			lineOffset++
		} else if doCHeader && *cfmt {
			w.Write(unsignedChar)
			w.Write(cVariable)
			w.Write(brackets)
			doCHeader = false
		}

		if *binary {
			// Binary values
			for i := 0; i < n; i++ {
				binaryEncode(char, line[i:i+1])
				w.Write(char)

				if i%2 == 1 {
					w.Write(space)
				}
			}
		} else if *cfmt {
			// C values
			w.Write(space)
			for i := 0; i < n && i != 0; i++ {
				w.Write(hexPrefix)
				hexEncode(char, line[i:i+1], caps)
				w.Write(char)
				w.Write(comma)

				if i%2 == 1 {
					w.Write(space)
				}
			}
		} else if *postscript {
			// Post script values
			// Basically just raw hex output
			for i := 0; i < n; i++ {
				hexEncode(char, line[i:i+1], caps)
				w.Write(char)
			}
		} else {
			// Hex values -- default xxd FILE output
			for i := 0; i < n; i++ {
				hexEncode(char, line[i:i+1], caps)
				w.Write(char)

				if i%2 == 1 {
					w.Write(space)
				}
			}
		}

		if n < len(line) {
			for i := n; i < len(line); i++ {
				w.Write(doubleSpace)

				if i%2 == 1 {
					w.Write(space)
				}
			}
		}

		w.Write(space)

		if *binary || !odd {
			// Character values
			b := line[:n]
			if *ebcdic {
				for _, c := range b {
					if c >= ebcdicOffset {
						e := ebcdicTable[c-ebcdicOffset : c-ebcdicOffset+1]
						if e[0] > 0x1f && e[0] < 0x7f {
							w.Write(e)
						} else {
							w.Write(dot)
						}
					} else {
						w.Write(dot)
					}
				}
			} else {
				for i, c := range b {
					if c > 0x1f && c < 0x7f {
						w.Write(line[i : i+1])
					} else {
						w.Write(dot)
					}
				}
			}
		}

		w.Write(newLine)
		w.Flush()
	}
	return nil
}

func main() {
	flag.Parse()
	if len(flag.Args()) > 1 {
		panic("too many args")
	}
	file := flag.Args()[0]

	f, err := os.Open(file)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	if err := xxd(f, file); err != nil {
		panic(err)
	}
}
