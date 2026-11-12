package gallery

import "hellobro/ink/widget"

// A book, for the table, the grid and the strip.
type book struct {
	Title, Author string
	Year, Pages   int
}

var books = []book{
	{"Pride and Prejudice", "Jane Austen", 1813, 432},
	{"Emma", "Jane Austen", 1815, 474},
	{"Frankenstein", "Mary Shelley", 1818, 280},
	{"Moby-Dick", "Herman Melville", 1851, 635},
	{"Bleak House", "Charles Dickens", 1853, 1017},
	{"Great Expectations", "Charles Dickens", 1861, 544},
	{"Madame Bovary", "Gustave Flaubert", 1857, 329},
	{"Les Misérables", "Victor Hugo", 1862, 1463},
	{"Alice's Adventures in Wonderland", "Lewis Carroll", 1865, 96},
	{"Crime and Punishment", "Fyodor Dostoevsky", 1866, 551},
	{"War and Peace", "Leo Tolstoy", 1869, 1225},
	{"Middlemarch", "George Eliot", 1872, 880},
	{"Anna Karenina", "Leo Tolstoy", 1878, 864},
	{"The Brothers Karamazov", "Fyodor Dostoevsky", 1880, 796},
	{"Treasure Island", "Robert Louis Stevenson", 1883, 292},
	{"Huckleberry Finn", "Mark Twain", 1884, 366},
	{"The Strange Case of Dr Jekyll", "Robert Louis Stevenson", 1886, 141},
	{"A Study in Scarlet", "Arthur Conan Doyle", 1887, 108},
	{"The Picture of Dorian Gray", "Oscar Wilde", 1890, 254},
	{"The Time Machine", "H. G. Wells", 1895, 118},
	{"Dracula", "Bram Stoker", 1897, 418},
	{"Heart of Darkness", "Joseph Conrad", 1899, 96},
	{"The Hound of the Baskervilles", "Arthur Conan Doyle", 1902, 256},
	{"The Call of the Wild", "Jack London", 1903, 232},
	{"The Wind in the Willows", "Kenneth Grahame", 1908, 302},
	{"The Secret Garden", "Frances Hodgson Burnett", 1911, 331},
	{"Ulysses", "James Joyce", 1922, 730},
	{"Mrs Dalloway", "Virginia Woolf", 1925, 194},
	{"The Great Gatsby", "F. Scott Fitzgerald", 1925, 180},
	{"To the Lighthouse", "Virginia Woolf", 1927, 209},
	{"Orlando", "Virginia Woolf", 1928, 333},
	{"The Sound and the Fury", "William Faulkner", 1929, 326},
	{"Brave New World", "Aldous Huxley", 1932, 311},
	{"Of Mice and Men", "John Steinbeck", 1937, 107},
	{"The Grapes of Wrath", "John Steinbeck", 1939, 464},
	{"Animal Farm", "George Orwell", 1945, 112},
	{"Nineteen Eighty-Four", "George Orwell", 1949, 328},
	{"The Old Man and the Sea", "Ernest Hemingway", 1952, 127},
	{"Fahrenheit 451", "Ray Bradbury", 1953, 158},
	{"Lolita", "Vladimir Nabokov", 1955, 336},
}

var bookCols = []string{"Title", "Author", "Year", "Pages"}

// tree is what the tree shows: folders, as the apps keep them.
var tree = []widget.Node{
	{Key: "nb", Title: "Notebooks", Sub: "14", Kids: []widget.Node{
		{Key: "nb/diary", Title: "Diary", Sub: "3", Kids: []widget.Node{
			{Key: "nb/diary/1112", Title: "Nov 12", Sub: "4 pages"},
			{Key: "nb/diary/1111", Title: "Nov 11", Sub: "2 pages"},
			{Key: "nb/diary/1110", Title: "Nov 10", Sub: "7 pages"},
		}},
		{Key: "nb/work", Title: "Work", Sub: "6", Kids: []widget.Node{
			{Key: "nb/work/meet", Title: "Meetings", Sub: "3", Kids: []widget.Node{
				{Key: "nb/work/meet/mon", Title: "Monday's", Sub: "1 page"},
				{Key: "nb/work/meet/plan", Title: "The plan", Sub: "5 pages"},
				{Key: "nb/work/meet/retro", Title: "Looking back", Sub: "2 pages"},
			}},
			{Key: "nb/work/ink", Title: "ink, the sketches", Sub: "9 pages"},
			{Key: "nb/work/todo", Title: "To do", Sub: "1 page"},
		}},
		{Key: "nb/maths", Title: "Maths", Sub: "12 pages"},
		{Key: "nb/draw", Title: "Drawings", Sub: "20 pages"},
		{Key: "nb/empty", Title: "Nothing yet", Branch: true},
	}},
	{Key: "bk", Title: "Books", Sub: "40", Kids: []widget.Node{
		{Key: "bk/fic", Title: "Fiction", Sub: "4", Kids: []widget.Node{
			{Key: "bk/fic/moby", Title: "Moby-Dick", Sub: "EPUB"},
			{Key: "bk/fic/emma", Title: "Emma", Sub: "EPUB"},
			{Key: "bk/fic/dracula", Title: "Dracula", Sub: "EPUB"},
			{Key: "bk/fic/orlando", Title: "Orlando", Sub: "EPUB"},
		}},
		{Key: "bk/man", Title: "Manuals", Sub: "2", Kids: []widget.Node{
			{Key: "bk/man/elipsa", Title: "Elipsa's guide", Sub: "PDF"},
			{Key: "bk/man/game", Title: "A game's manual", Sub: "PDF"},
		}},
	}},
	{Key: "al", Title: "Alpine", Sub: "/root", Kids: []widget.Node{
		{Key: "al/notes", Title: "notes.md", Sub: "2 KB"},
		{Key: "al/src", Title: "src", Sub: "1", Kids: []widget.Node{
			{Key: "al/src/hello", Title: "hello.go", Sub: "1 KB"},
		}},
	}},
	{Key: "readme", Title: "README", Sub: "a file at the top"},
}

// The page of text the scroll box moves.
var longText = []string{
	"A list on e-ink is turned a page at a time: each page is drawn once, in grays, and stays. That's what the paged list does, and what most of the apps want.",
	"Some things want to move under the finger, though: a long page of text to read on a bit, a table to look down, a tree with its folders open. Here the finger holds the page and it goes where the finger goes.",
	"Each move is a frame in black and white, sent in the fast waveform at once: the panel shows it in a tenth of a second or so. The grays wait. When the finger lifts, the frame after puts what moved back in grays, in one update.",
	"The panel can't keep up with every move: while one update's being drawn, the next ones pile up and go as one. So a fast drag skips frames, as it should, and a slow one follows the finger closely.",
	"A tap is still a tap: the list only takes the finger once it has moved a little along the list. Until then the row under it is pressed, as ever, and lets go if the list takes it.",
	"The rows are laid out only as they show, so the list can be long. One item longer than the box, as this page of text is, moves as well as many short ones.",
	"PageDown and PageUp move it nearly a page, so a line stays in sight from the page before.",
	"Why black and white while it moves? The fast waveform takes a little over a tenth of a second and leaves no flash, but it can only make a pixel black or white. Text's smoothed edges come out rough for a moment, which a moving page hides well enough.",
	"Why grays after? The grayscale waveform takes nearly half a second, too slow to follow a finger, but it draws the letters' edges as they are. Once the page rests, one such update over what moved makes it look as it would have if it had been drawn there in the first place.",
	"And ghosting? Each update leaves a trace of what was there before, a little more after many fast ones. The cleanup in grays takes most of it away; now and then the app flashes the whole screen, black then white, which takes the rest.",
	"A list by the page asks none of this of the panel: each page is one update in grays. It's still the better list for reading through many rows, one page after the other. A list under the finger is the better one for looking for something: a quick move, a look, a little back.",
	"Both are in the gallery: the list on the Lists page goes either way, by its switch. The table and the tree move under the finger; the grid of covers turns by the page.",
	"That's all there is to it. The rest is the panel's: black and white at once, grays after.",
}

// The keys, as the page of text ends.
var keyRows = [][2]string{
	{"Tab, Shift+Tab", "the focus on, and back"},
	{"↑ ↓ ← →", "the focus, or within what has it"},
	{"Enter, Space", "presses what has the focus"},
	{"↓ on a dropdown", "opens it"},
	{"Esc", "closes what's open; at the top, quits"},
	{"PageDown, PageUp", "the page on and back"},
	{"Home, End", "a dropdown's first, its last"},
	{"→ ← on a folder", "opens it, closes it"},
	{"Space on a range", "the other end"},
	{"letters", "into the field in focus"},
}

// The text area's text, at first: each kind of paragraph.
const noteText = `# Trip to the sea
Three days by the water, the Kobo in the bag, nothing else to read.
## To pack
- [x] the Kobo, charged
- [x] its pen
- [ ] a jumper for the evenings
- [ ] the charger, this time
## What to read
- the second half of Middlemarch
- a book of short stories, for the train
- the notes from last week, to tidy up
> The sea, the sea, as the book says: nothing to do but look at it.
## Notes
Words typed here wrap to the room they have. Tap a box to tick it; Back at the start of a list's line takes it out of the list; Enter on an empty one ends it. The buttons over the text make the line the caret's in a title, a heading, a list, a to-do, a quote, or plain text again.`
