package main

// What plays: a queue of tracks, from an album, a folder with all its
// subfolders, or a playlist. It plays in order, or shuffled (the track
// playing stays first, the rest come in a random order); at its end it
// stops, or goes round again, or plays the one track over and over.

import (
	"math/rand/v2"
	"slices"
)

type repeatMode int

const (
	repeatOff repeatMode = iota
	repeatAll
	repeatOne
)

var repeatNames = []string{"off", "all", "one"}

const (
	qAlbum    = "album"
	qFolder   = "folder"
	qPlaylist = "playlist"
)

type queue struct {
	kind   string   // qAlbum, qFolder, qPlaylist
	src    string   // the album's or the folder's path, or the playlist file
	name   string   // as shown
	tracks []string // the files, in their own order
	order  []int    // the order they play in: indexes into tracks
	pos    int      // what's on: an index into order
}

func newQueue(kind, src, name string, tracks []string) *queue {
	q := &queue{kind: kind, src: src, name: name, tracks: tracks}
	q.unshuffle()
	return q
}

func (q *queue) len() int { return len(q.tracks) }

// cur is the index in tracks of what's on.
func (q *queue) cur() int {
	if q.pos < 0 || q.pos >= len(q.order) {
		return -1
	}
	return q.order[q.pos]
}

// path is the file that's on, or "".
func (q *queue) path() string {
	if i := q.cur(); i >= 0 {
		return q.tracks[i]
	}
	return ""
}

// jump makes track i of tracks the one on.
func (q *queue) jump(i int) {
	if p := slices.Index(q.order, i); p >= 0 {
		q.pos = p
	}
}

// find is the index in tracks of path, or -1.
func (q *queue) find(path string) int { return slices.Index(q.tracks, path) }

// shuffle puts the tracks in a random order: the one on first if first,
// else any.
func (q *queue) shuffle(first bool) {
	cur := q.cur()
	q.order = rand.Perm(len(q.tracks))
	if first && cur >= 0 {
		p := slices.Index(q.order, cur)
		q.order[0], q.order[p] = q.order[p], q.order[0]
	}
	q.pos = 0
}

// unshuffle puts them back in order, keeping the one on.
func (q *queue) unshuffle() {
	cur := q.cur()
	q.order = make([]int, len(q.tracks))
	for i := range q.order {
		q.order[i] = i
	}
	q.pos = max(0, cur)
}

func (q *queue) shuffled() bool {
	for i, t := range q.order {
		if i != t {
			return true
		}
	}
	return false
}

// step goes d tracks on (or back), as repeat says at the ends; false if
// it's past the end (or the start) and stays put. Shuffled, going round
// again shuffles anew.
func (q *queue) step(d int, rep repeatMode, shuffled bool) bool {
	if len(q.order) == 0 {
		return false
	}
	if rep == repeatOne {
		return true
	}
	p := q.pos + d
	if p >= 0 && p < len(q.order) {
		q.pos = p
		return true
	}
	if rep != repeatAll {
		return false
	}
	if shuffled && p >= len(q.order) {
		last := q.cur()
		q.order = rand.Perm(len(q.tracks))
		if len(q.order) > 1 && q.order[0] == last { // not the same one twice running
			q.order[0], q.order[1] = q.order[1], q.order[0]
		}
		q.pos = 0
		return true
	}
	q.pos = (p%len(q.order) + len(q.order)) % len(q.order)
	return true
}
