package main

// Pollock is a simple, low-level (assembly like) programming language which executes on a stack-based virtual machine.
// It supports a limited set of instructions and the compiler generates a png image file as output.
// The image is a grid of cells, each cell represents an instruction in the program.
// Pollock image format definition
// First pixel of the first cell: [major version, minor version, cellsize]
// First pixel of the second cell: [tnol % 16777216, tnol % 65536, tnol % 256], where tnol = total number of lines - 2
// We do not need to count the first two elements, since they are the metainfo
//
// If the number of lines is 0 or 1, we have a vertical image, due to flooring sqrt!
// After acquiring the metadata, any pixel is good from the cell to get the 3 channel instructions (v1.0)
//
// Pollock assembly allows the usage of labels of 7 chars, starting with a capital letter, followed by more capital letters and digits.
//
// Todo:
// - Add support for labels

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log"
	"math"
	"os"
	"regexp"
	"strconv"
)

var pushOpWOArg = errors.New("Push operation without argument")
var pushOpArgOutOfRange = errors.New("Push operation argument out of range")
var pushOpArgInvalid = errors.New("Push operation argument invalid")
var unknownOp = errors.New("Unknown operation")
var silent bool

func colChannel(channel int) string {
	switch channel {
	case 0:
		return "R"
	case 1:
		return "G"
	case 2:
		return "B"
	default:
		return "Unknown channel"
	}
}

func tokenize(instr []byte) (uint8, error) {
	//fmt.Println("Tokenizing instruction:", string(instr))
	// First catching the special case of "pusha" instruction
	if string(instr) == "pusha" {
		return 0b1101_1100, nil
	}
	// Then catching the special case of "push" instruction
	// The push instruction must have an argument, so we need to check for it
	pushOp, _ := regexp.Compile(`^push.*$`)
	labelArg, _ := regexp.Compile(`^[A-Z][A-Z0-9]{0,6}(_[1234])?$`)

	if pushOp.Match(instr) {
		pushArg := instr[4:]
		if len(pushArg) == 0 {
			// If there is no argument, we return an error
			return 0b0000_0000, pushOpWOArg
		} else {
			if labelArg.Match(pushArg) {
				fmt.Println("   Label argument detected:", string(pushArg))
			} else {
				pushArgUint, err := strconv.ParseUint(string(pushArg), 10, 8)
				if err == nil {
					pushArgUint8 := uint8(pushArgUint)
					if pushArgUint8 >= 0b0000_0000 && pushArgUint8 <= 0b0111_1111 {
						// If the argument is a number in the range of 0-127, we return it
						return pushArgUint8, nil
					} else {
						// If the argument is out of range, we return an error
						return 0b0000_0000, pushOpArgOutOfRange
					}
				} else {
					// If the argument is not a number, we return an error
					return 0b0000_0000, pushOpArgInvalid
				}

			}
		}
	}
	// Handling the rest of the instructions
	// We use a switch statement to match the instruction and return the corresponding token
	switch string(instr) {
	case "add":
		return 0b1000_0000, nil
	case "sub":
		return 0b1000_0100, nil
	case "mul":
		return 0b1000_1000, nil
	case "div":
		return 0b1000_1100, nil
	case "rem":
		return 0b1001_0000, nil
	case "pop":
		return 0b1001_0100, nil
	case "swap":
		return 0b1001_1000, nil
	case "dup":
		return 0b1001_1100, nil
	case "rot":
		return 0b1010_0000, nil
	case "not":
		return 0b1010_0100, nil
	case "or":
		return 0b1010_1000, nil
	case "and":
		return 0b1010_1100, nil
	case "gt":
		return 0b1011_0000, nil
	case "eq":
		return 0b1011_0100, nil
	case "lt":
		return 0b1011_1000, nil
	case "nop":
		return 0b1011_1100, nil
	case "halt":
		return 0b1100_0000, nil
	case "jmpz":
		return 0b1100_0100, nil
	case "jmpnz":
		return 0b1100_1000, nil
	case "outc":
		return 0b1100_1100, nil
	case "inc":
		return 0b1101_0000, nil
	case "outi":
		return 0b1101_0100, nil
	case "ini":
		return 0b1101_1000, nil
	case "waita":
		return 0b1110_0000, nil
	case "neg":
		return 0b1110_0100, nil
	case "shl":
		return 0b1110_1000, nil
	case "shr":
		return 0b1110_1100, nil
	// Unknown instruction, replacing it with a nop and raising an error
	default:
		return 0b1011_1100, unknownOp
	}
}

func logWrapper(msg string) {
	if !silent {
		log.Println(msg)
	}
}

func main() {
	var filename string
	var dryrun bool
	var bytearray bool
	var cellsize int
	var outputfile string
	var progline int = 0
	var token uint8
	var maxX, maxY int

	type progarray struct {
		r []uint8
		g []uint8
		b []uint8
	}

	const (
		VMAJOR = 1
		VMINOR = 0
	)

	// Parsing command line flags
	flag.StringVar(&filename, "f", "", "Path to the file, mandatory")
	flag.BoolVar(&dryrun, "d", false, "Run in dry run mode, default is false")
	flag.BoolVar(&silent, "s", false, "Run in silent mode, default is false")
	flag.BoolVar(&bytearray, "b", false, "Output only a bytes in text format, default is false")
	flag.StringVar(&outputfile, "o", "", "Output file name, default is same as input file")
	flag.IntVar(&cellsize, "c", 10, "Cell size in bytes, must be between 2 and 50, default value is 10")
	flag.Parse()

	logWrapper("Pollock started")
	logWrapper("Flags parsed")
	logWrapper(fmt.Sprint(" Version: ", VMAJOR, ".", VMINOR))
	logWrapper(fmt.Sprint(" Filename: ", filename))
	logWrapper(fmt.Sprint(" Cell size: ", cellsize))
	logWrapper(fmt.Sprint(" Dry run: ", dryrun))
	logWrapper(fmt.Sprint(" Silent: ", silent))
	logWrapper(fmt.Sprint(" Byte array: ", bytearray))
	logWrapper(fmt.Sprint(" Output file: ", outputfile))

	// Filename must exist, must have a .plk extension, cellsize must be between 2 and 50, outputfile is optional
	if len(filename) == 0 {
		log.Fatalln("Fatal error: Filename is required.")
	}
	if filename[len(filename)-4:] != ".plk" {
		log.Fatalln("Fatal error: File must have a .plk extension.")
	}
	if cellsize < 2 || cellsize > 50 {
		log.Fatalln("Fatal error: Cell size must be between 2 and 100.")
	}
	if len(outputfile) == 0 {
		outputfile = filename[0:len(filename)-4] + ".png"
		logWrapper(fmt.Sprint("Output file not specified, using default: ", outputfile))
	}
	logWrapper(fmt.Sprint("Reading file: ", filename))
	file, err := os.ReadFile(filename)
	if err != nil {
		log.Fatalln("Fatal error:", "\"", err, "\"")
	}

	// Building regexps
	commentLine, _ := regexp.Compile(`(?m)^\s*#.*$`)
	whitespace, _ := regexp.Compile(`\s+`)
	comment, _ := regexp.Compile(`;?(#.*)?$`)
	emptyLine, _ := regexp.Compile(`(?m)^$`)
	colon, _ := regexp.Compile(`:`)
	label, err := regexp.Compile(`[A-Z][A-Z0-9]{0,6}`)

	logWrapper("Initializing program array")
	fileLines := bytes.Split(file, []byte("\n"))
	fileLinesLen := len(fileLines)
	// Initialize the program array (length of fileLines) with "nop" instructions
	// fileLines size is enough to hold all the instructions even if there are no empty or comment lines
	program := progarray{r: make([]uint8, fileLinesLen), g: make([]uint8, fileLinesLen), b: make([]uint8, fileLinesLen)}
	for i := 0; i < fileLinesLen; i++ {
		program.r[i] = 0b1011_1100
		program.g[i] = 0b1011_1100
		program.b[i] = 0b1011_1100
	}
	logWrapper(fmt.Sprint("Program array initialized with ", fileLinesLen, " nop instructions."))
	for lineno, lineStr := range fileLines {
		//fmt.Println("Line number:", lineno, "Line string:", string(lineStr))
		if emptyLine.Match(lineStr) {
			// This is an empty line, skipping it
			logWrapper(fmt.Sprint("Empty line detected at line: ", lineno+1, ". Skipping."))
			continue
		}
		if lineStr[0] != 13 {
			if commentLine.Match(lineStr) {
				// This is a comment line, skipping it
				continue
			} else {
				lineStr = comment.ReplaceAll(lineStr, []byte(""))
				lineStr = whitespace.ReplaceAll(lineStr, []byte(""))
				if colon.Match(lineStr) {
					colonMatches := colon.FindAllIndex(lineStr, -1)
					if len(colonMatches) > 1 {
						log.Fatalln("Syntax error. Multiple labels detected in line:", lineno+1, ".")
					} else {
						labeledItem := bytes.Split(lineStr, []byte(":"))
						if len(labeledItem[0]) >= 1 && len(labeledItem[0]) <= 7 && label.Match(labeledItem[0]) {
							fmt.Println("   Label detected:", string(labeledItem[0]), "in line:", lineno+1, ".")
						} else {
							if len(labeledItem[0]) == 0 {
								log.Fatalln("Syntax error. Empty label detected in line:", lineno+1, ".")
							} else {
								log.Fatalln("Syntax error. Invalid label detected: \"", string(labeledItem[0]), "\" in line:", lineno+1, ".")
							}
						}
						lineStr = labeledItem[1]
					}
				}
				instrItems := bytes.Split(lineStr, []byte(";"))
				//fmt.Println("Line:", lineno+1, "Instruction items:", instrItems)
				//fmt.Println("Instruction items:", instrItems)
				prevInstrNum := 0
				for instrNum, instr := range instrItems {
					prevInstrNum = instrNum
					if instrNum <= 2 {
						if len(instr) > 0 {
							//fmt.Println("Line:", lineno+1, "Channel:", colChannel(instrNum), "Instruction:", string(instr))
							token, err = tokenize(instr)
							if err != nil {
								switch err {
								case unknownOp:
									logWrapper(fmt.Sprint("Unknown instruction \"", string(instr), "\" in line: ", lineno+1, ", position: ", colChannel(instrNum), ". Replacing with nop."))
								case pushOpWOArg:
									logWrapper(fmt.Sprint("Push operation without argument in line: ", lineno+1, ", position: ", colChannel(instrNum), ". Using zero as a value."))
								case pushOpArgOutOfRange:
									logWrapper(fmt.Sprint("Push operation argument is out of range in line: ", lineno+1, ", position: ", colChannel(instrNum), ". Using zero as a value."))
								case pushOpArgInvalid:
									logWrapper(fmt.Sprint("Push operation argument is invalid in line: ", lineno+1, ", position: ", colChannel(instrNum), ". Using zero as a value."))
								}
							}
						} else {
							logWrapper(fmt.Sprint("Empty instruction in line: ", lineno+1, ", position: ", colChannel(instrNum), ". Using nop."))
							token, _ = tokenize([]byte("nop"))
						}
						switch instrNum {
						case 0:
							program.r[progline] = token
						case 1:
							program.g[progline] = token
						case 2:
							program.b[progline] = token
						}
					} else {
						if len(instr) > 0 {
							// This is an extra instruction, we will skip it
							logWrapper(fmt.Sprint("Dropped extra text \"", string(instr), "\" in line: ", lineno+1, "."))
						}
					}
				}
				// If we have only one or two instructions, we need to fill the other channels with nop
				switch prevInstrNum {
				case 0:
					logWrapper(fmt.Sprint("Missing instruction in line: ", lineno+1, ", position: ", colChannel(prevInstrNum+1), ". Using nop."))
					program.g[progline], _ = tokenize([]byte("nop"))
					logWrapper(fmt.Sprint("Missing instruction in line: ", lineno+1, ", position: ", colChannel(prevInstrNum+2), ". Using nop."))
					program.b[progline], _ = tokenize([]byte("nop"))
				case 1:
					logWrapper(fmt.Sprint("Missing instruction in line: ", lineno+1, ", position: ", colChannel(prevInstrNum+1), ". Using nop."))
					program.b[progline], _ = tokenize([]byte("nop"))
				}
				progline++
			}
		} else {
			// This is an empty line, skipping it
			logWrapper(fmt.Sprint("Empty line detected at line: ", lineno+1, ". Skipping."))
		}
	}
	logWrapper(fmt.Sprint("Program array filled with ", progline, " instructions."))
	if !dryrun {
		switch progline {
		case 1:
			maxX = 1
			maxY = 3
		case 2:
			maxX = 2
			maxY = 2
		default:
			maxX = int(math.Floor(math.Sqrt(float64(progline + 2))))
			maxX2 := maxX * maxX
			maxY = 0
			if maxX2 == progline {
				maxY = maxX
			} else {
				maxY = int(math.Ceil(float64(progline+2) / float64(maxX)))
			}
		}
		logWrapper(fmt.Sprint("X size: ", maxX, ", Y size: ", maxY))
		imageRectangle := image.Rect(0, 0, maxX*cellsize, maxY*cellsize)
		imagePix := image.NewRGBA(imageRectangle)
		// Inserting the version number in the first cell
		xCoord, yCoord := 0, 0
		for i := 0; i < cellsize; i++ {
			for j := 0; j < cellsize; j++ {
				imagePix.Set(xCoord+i, yCoord+j, color.RGBA{R: uint8(VMAJOR), G: uint8(VMINOR), B: uint8(cellsize), A: 255})
			}
		}
		xCoord++
		if xCoord >= maxX {
			xCoord = 0
			yCoord++
		}
		// Inserting the total size in the second cell
		for i := 0; i < cellsize; i++ {
			for j := 0; j < cellsize; j++ {
				imagePix.Set(xCoord*cellsize+i, yCoord*cellsize+j, color.RGBA{R: uint8((progline >> 16) % 256), G: uint8((progline >> 8) % 256), B: uint8(progline % 256), A: 255})
			}
		}
		xCoord++
		if xCoord >= maxX {
			xCoord = 0
			yCoord++
		}
		// Now we can fill the rest of the cells with the program instructions
		for k := 0; k < progline; k++ {
			for i := 0; i < cellsize; i++ {
				for j := 0; j < cellsize; j++ {
					imagePix.Set(xCoord*cellsize+i, yCoord*cellsize+j, color.RGBA{R: program.r[k], G: program.g[k], B: program.b[k], A: 255})
				}
			}
			xCoord++
			if xCoord >= maxX {
				xCoord = 0
				yCoord++
			}
		}
		// Creating the output file
		logWrapper(fmt.Sprint("Creating img file: ", outputfile))
		f, err := os.Create(outputfile)
		if err == nil {
			if err := png.Encode(f, imagePix); err != nil {
				f.Close()
				log.Fatalln("Fatal encode error:", "\"", err, "\"")
			}
			if err := f.Close(); err != nil {
				log.Fatalln("Fatal close error:", "\"", err, "\"")
			}
		} else {
			log.Fatalln("Fatal create error:", "\"", err, "\"")
		}
		// If we have a bytearray flag, we will print the program array in a text format
		if bytearray {
			for i := 0; i < progline; i++ {
				fmt.Println("Line:", i+1, "R:", program.r[i], "G:", program.g[i], "B:", program.b[i])
			}
		}
	}
}
