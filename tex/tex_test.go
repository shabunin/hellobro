package tex

import (
	"image"
	"image/draw"
	"image/png"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// samples are formulas as models write them.
var samples = []struct {
	src     string
	display bool
}{
	{`x^2 + y^2 = r^2`, false},
	{`\frac{a}{b} + \sqrt{2} \cdot \pi \approx 4.44`, false},
	{`\int_0^1 x^2\,dx = \frac{1}{3}`, true},
	{`\sum_{n=1}^{\infty} \frac{1}{n^2} = \frac{\pi^2}{6}`, true},
	{`\lim_{x \to 0} \frac{\sin x}{x} = 1`, true},
	{`x = \frac{-b \pm \sqrt{b^2 - 4ac}}{2a}`, true},
	{`\left( \frac{a+b}{c} \right)^{2} \le \left[ \sum_{i} x_i \right]`, true},
	{`A = \begin{pmatrix} 1 & 2 \\ 3 & 4 \end{pmatrix}, \quad \det A = -2`, true},
	{`|x| = \begin{cases} x, & x \ge 0 \\ -x, & x < 0 \end{cases}`, true},
	{`\begin{aligned} (a+b)^2 &= a^2 + 2ab + b^2 \\ &\neq a^2 + b^2 \end{aligned}`, true},
	{`e^{i\pi} + 1 = 0, \quad \hat{x}, \vec{v}, \bar{z}, \overline{AB}, \tilde{n}, \dot{x}`, false},
	{`\mathbb{R}^n, \mathcal{L}, \mathbf{F} = m\mathbf{a}, \text{скорость } v = \frac{s}{t}`, false},
	{`\nabla \cdot \vec{E} = \frac{\rho}{\varepsilon_0}, \quad \alpha\beta\gamma\Delta\Omega\theta\varphi`, false},
	{`\sqrt[3]{x+1} \Rightarrow \forall x \in \mathbb{N}\; \exists y : y > x`, false},
	{`\binom{n}{k} = \frac{n!}{k!(n-k)!}`, true},
	{`f'(x) = \lim_{h\to 0}\frac{f(x+h)-f(x)}{h}`, false},
	{`\underbrace{1+1+\cdots+1}_{n} = n, \quad a \xrightarrow{f} b`, true},
	{`\oint_C \mathbf{F}\cdot d\mathbf{r} = \iint_S (\nabla\times\mathbf{F})\cdot d\mathbf{S}`, true},
	{`P(A\mid B) = \frac{P(B\mid A)\,P(A)}{P(B)}`, true},
	{`\frac{1}{1+\frac{1}{1+\frac{1}{x}}}`, true},
	{`\left\{ x \in \mathbb{R} \;\middle|\; x^2 < 2 \right\}`, true},
	{`\boxed{E = mc^2} \quad \cancel{x} \quad 50\% \quad \unknowncmd`, false},
}

func TestRender(t *testing.T) {
	out := os.Getenv("TEX_OUT")
	var imgs []*image.Gray
	for i, s := range samples {
		r := Render(s.src, s.display, 40)
		b := r.Img.Bounds()
		if b.Dx() < 20 || b.Dy() < 20 || r.Baseline <= 0 || r.Baseline >= b.Dy() {
			t.Errorf("%d %q: %v, baseline %d", i, s.src, b, r.Baseline)
		}
		ink := 0
		for _, p := range r.Img.Pix {
			if p < 128 {
				ink++
			}
		}
		if ink < 50 {
			t.Errorf("%d %q: no ink", i, s.src)
		}
		if (len(r.Unknown) > 0) != strings.Contains(s.src, `\unknowncmd`) {
			t.Errorf("%d %q: unknown %v", i, s.src, r.Unknown)
		}
		imgs = append(imgs, r.Img)
	}
	if out != "" {
		w, h := 0, 0
		for _, m := range imgs {
			w, h = max(w, m.Bounds().Dx()), h+m.Bounds().Dy()+20
		}
		sheet := image.NewGray(image.Rect(0, 0, w+40, h+20))
		draw.Draw(sheet, sheet.Bounds(), image.White, image.Point{}, draw.Src)
		y := 20
		for _, m := range imgs {
			draw.Draw(sheet, m.Bounds().Add(image.Pt(20, y)), m, image.Point{}, draw.Src)
			y += m.Bounds().Dy() + 20
		}
		f, err := os.Create(filepath.Join(out, "tex.png"))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		png.Encode(f, sheet)
	}
}

// TestRenderJunk: whatever's written, it ends, and draws something no
// bigger than it may.
func TestRenderJunk(t *testing.T) {
	bits := []string{`\frac`, `{`, `}`, `^`, `_`, `\left(`, `\right)`, `\sqrt[`, `]`, `&`, `\\`, `\begin{matrix}`,
		`\end{matrix}`, `x`, ` `, `\text{`, `$`, `\over`, `\not`, `\hspace{999em}`, `'`, `\middle|`, `\begin{cases}`,
		`\overbrace`, `\big`, `\kern`, `\mathbf`, `\`, `%`, `\begin{array}{`}
	rnd := rand.New(rand.NewSource(1))
	for i := range 3000 {
		var b strings.Builder
		for range rnd.Intn(30) {
			b.WriteString(bits[rnd.Intn(len(bits))])
		}
		src := b.String()
		func() {
			defer func() {
				if e := recover(); e != nil {
					t.Fatalf("%d %q: %v", i, src, e)
				}
			}()
			r := Render(src, i%2 == 0, 30)
			if r.Img.Bounds().Dx() > maxSide || r.Img.Bounds().Dy() > maxSide {
				t.Fatalf("%q: %v", src, r.Img.Bounds())
			}
		}()
	}
	deep := strings.Repeat(`\frac{`, 500) + "x" + strings.Repeat("}{y}", 500)
	Render(deep, true, 30)
}
