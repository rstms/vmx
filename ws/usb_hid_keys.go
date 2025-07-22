package ws

/**
 * USB HID Keyboard scan codes as per USB spec 1.11
 * plus some additional codes
 *
 * Created by MightyPork, 2016
 * Public domain
 *
 * Adapted from:
 * https://source.android.com/devices/input/keyboard-devices.html
 *
 * Further adapted subset for golang from:
 * https://gist.github.com/MightyPork/6da26e382a7ad91b5496ee55fdc73db2
 *
 */

/**
 * Modifier masks - used for the first byte in the HID report.
 * NOTE: The second byte in the report is reserved, 0x00
 */

const KEY_MOD_NONE = 0x00
const KEY_MOD_LCTRL = 0x01
const KEY_MOD_LSHIFT = 0x02
const KEY_MOD_LALT = 0x04
const KEY_MOD_LMETA = 0x08
const KEY_MOD_RCTRL = 0x10
const KEY_MOD_RSHIFT = 0x20
const KEY_MOD_RALT = 0x40
const KEY_MOD_RMETA = 0x80

type HID struct {
	Code     uint32
	Modifier uint32
}

var HIDMap map[rune]HID = map[rune]HID{

	'\x08': {0x2a, KEY_MOD_NONE},  // Keyboard DELETE (Backspace)
	'\x09': {0x2b, KEY_MOD_NONE},  // Keyboard Tab
	'\x0a': {0x28, KEY_MOD_NONE},  // \n newline ENTER
	'\x0b': {0x0e, KEY_MOD_LCTRL}, // CTRL-K
	'\x0c': {0x0f, KEY_MOD_LCTRL}, // CTRL-L
	'\x0d': {0x10, KEY_MOD_LCTRL}, // CTRL-M \r return ENTER

	'\x1b': {0x29, KEY_MOD_NONE}, // Keyboard ESCAPE

	' ':    {0x2c, KEY_MOD_NONE},   // Keyboard Spacebar
	'!':    {0x1e, KEY_MOD_LSHIFT}, // Keyboard 1 and !
	'\x22': {0x34, KEY_MOD_LSHIFT}, // Keyboard ' and "
	'#':    {0x20, KEY_MOD_LSHIFT}, // Keyboard 3 and //
	'$':    {0x21, KEY_MOD_LSHIFT}, // Keyboard 4 and $
	'%':    {0x22, KEY_MOD_LSHIFT}, // Keyboard 5 and %
	'&':    {0x24, KEY_MOD_LSHIFT}, // Keyboard 7 and &
	'\x27': {0x34, KEY_MOD_NONE},   // Keyboard ' and "

	'(': {0x26, KEY_MOD_LSHIFT}, // Keyboard 9 and (
	')': {0x27, KEY_MOD_LSHIFT}, // Keyboard 0 and )
	'*': {0x25, KEY_MOD_LSHIFT}, // Keyboard 8 and *
	'+': {0x2e, KEY_MOD_LSHIFT}, // Keyboard = and +
	',': {0x36, KEY_MOD_NONE},   // Keyboard , and <
	'-': {0x2d, KEY_MOD_NONE},   // Keyboard - and _
	'.': {0x37, KEY_MOD_NONE},   // Keyboard . and >
	'/': {0x38, KEY_MOD_NONE},   // Keyboard / and ?

	'0': {0x27, KEY_MOD_NONE}, // Keyboard 0 and )
	'1': {0x1e, KEY_MOD_NONE}, // Keyboard 1 and !
	'2': {0x1f, KEY_MOD_NONE}, // Keyboard 2 and @
	'3': {0x20, KEY_MOD_NONE}, // Keyboard 3 and //
	'4': {0x21, KEY_MOD_NONE}, // Keyboard 4 and $
	'5': {0x22, KEY_MOD_NONE}, // Keyboard 5 and %
	'6': {0x23, KEY_MOD_NONE}, // Keyboard 6 and ^
	'7': {0x24, KEY_MOD_NONE}, // Keyboard 7 and &

	'8': {0x25, KEY_MOD_NONE},   // Keyboard 8 and *
	'9': {0x26, KEY_MOD_NONE},   // Keyboard 9 and (
	':': {0x33, KEY_MOD_LSHIFT}, // Keyboard ; and :
	';': {0x33, KEY_MOD_NONE},   // Keyboard ; and :
	'<': {0x36, KEY_MOD_LSHIFT}, // Keyboard , and <
	'=': {0x2e, KEY_MOD_NONE},   // Keyboard = and +
	'>': {0x37, KEY_MOD_LSHIFT}, // Keyboard . and >
	'?': {0x38, KEY_MOD_LSHIFT}, // Keyboard / and ?

	'@': {0x1f, KEY_MOD_LSHIFT}, // Keyboard 2 and @
	'A': {0x04, KEY_MOD_LSHIFT}, // Keyboard a and A
	'B': {0x05, KEY_MOD_LSHIFT}, // Keyboard b and B
	'C': {0x06, KEY_MOD_LSHIFT}, // Keyboard c and C
	'D': {0x07, KEY_MOD_LSHIFT}, // Keyboard d and D
	'E': {0x08, KEY_MOD_LSHIFT}, // Keyboard e and E
	'F': {0x09, KEY_MOD_LSHIFT}, // Keyboard f and F
	'G': {0x0a, KEY_MOD_LSHIFT}, // Keyboard g and G

	'H': {0x0b, KEY_MOD_LSHIFT}, // Keyboard h and H
	'I': {0x0c, KEY_MOD_LSHIFT}, // Keyboard i and I
	'J': {0x0d, KEY_MOD_LSHIFT}, // Keyboard j and J
	'K': {0x0e, KEY_MOD_LSHIFT}, // Keyboard k and K
	'L': {0x0f, KEY_MOD_LSHIFT}, // Keyboard l and L
	'M': {0x10, KEY_MOD_LSHIFT}, // Keyboard m and M
	'N': {0x11, KEY_MOD_LSHIFT}, // Keyboard n and N
	'O': {0x12, KEY_MOD_LSHIFT}, // Keyboard o and O

	'P': {0x13, KEY_MOD_LSHIFT}, // Keyboard p and P
	'Q': {0x14, KEY_MOD_LSHIFT}, // Keyboard q and Q
	'R': {0x15, KEY_MOD_LSHIFT}, // Keyboard r and R
	'S': {0x16, KEY_MOD_LSHIFT}, // Keyboard s and S
	'T': {0x17, KEY_MOD_LSHIFT}, // Keyboard t and T
	'U': {0x18, KEY_MOD_LSHIFT}, // Keyboard u and U
	'V': {0x19, KEY_MOD_LSHIFT}, // Keyboard v and V
	'W': {0x1a, KEY_MOD_LSHIFT}, // Keyboard w and W

	'X':    {0x1b, KEY_MOD_LSHIFT}, // Keyboard x and X
	'Y':    {0x1c, KEY_MOD_LSHIFT}, // Keyboard y and Y
	'Z':    {0x1d, KEY_MOD_LSHIFT}, // Keyboard z and Z
	'[':    {0x2f, KEY_MOD_NONE},   // Keyboard [ and {
	'\x5c': {0x31, KEY_MOD_NONE},   // Keyboard \ and |
	']':    {0x30, KEY_MOD_NONE},   // Keyboard ] and }
	'^':    {0x23, KEY_MOD_LSHIFT}, // Keyboard 6 and ^
	'_':    {0x2d, KEY_MOD_LSHIFT}, // Keyboard - and _

	'\x60': {0x35, KEY_MOD_NONE}, // Keyboard ` and ~
	'a':    {0x04, KEY_MOD_NONE}, // Keyboard a and A
	'b':    {0x05, KEY_MOD_NONE}, // Keyboard b and B
	'c':    {0x06, KEY_MOD_NONE}, // Keyboard c and C
	'd':    {0x07, KEY_MOD_NONE}, // Keyboard d and D
	'e':    {0x08, KEY_MOD_NONE}, // Keyboard e and E
	'f':    {0x09, KEY_MOD_NONE}, // Keyboard f and F
	'g':    {0x0a, KEY_MOD_NONE}, // Keyboard g and G

	'h': {0x0b, KEY_MOD_NONE}, // Keyboard h and H
	'i': {0x0c, KEY_MOD_NONE}, // Keyboard i and I
	'j': {0x0d, KEY_MOD_NONE}, // Keyboard j and J
	'k': {0x0e, KEY_MOD_NONE}, // Keyboard k and K
	'l': {0x0f, KEY_MOD_NONE}, // Keyboard l and L
	'm': {0x10, KEY_MOD_NONE}, // Keyboard m and M
	'n': {0x11, KEY_MOD_NONE}, // Keyboard n and N
	'o': {0x12, KEY_MOD_NONE}, // Keyboard o and O

	'p': {0x13, KEY_MOD_NONE}, // Keyboard p and P
	'q': {0x14, KEY_MOD_NONE}, // Keyboard q and Q
	'r': {0x15, KEY_MOD_NONE}, // Keyboard r and R
	's': {0x16, KEY_MOD_NONE}, // Keyboard s and S
	't': {0x17, KEY_MOD_NONE}, // Keyboard t and T
	'u': {0x18, KEY_MOD_NONE}, // Keyboard u and U
	'v': {0x19, KEY_MOD_NONE}, // Keyboard v and V
	'w': {0x1a, KEY_MOD_NONE}, // Keyboard w and W

	'x': {0x1b, KEY_MOD_NONE}, // Keyboard x and X
	'y': {0x1c, KEY_MOD_NONE}, // Keyboard y and Y
	'z': {0x1d, KEY_MOD_NONE}, // Keyboard z and Z

	'{': {0x2f, KEY_MOD_LSHIFT}, // Keyboard [ and {
	'|': {0x31, KEY_MOD_LSHIFT}, // Keyboard \ and |
	'}': {0x30, KEY_MOD_LSHIFT}, // Keyboard ] and }
	'~': {0x35, KEY_MOD_LSHIFT}, // Keyboard ` and ~
}
