package builtin

import (
	"errors"
	"fmt"
	"math"
	"strconv"
)

// Eval 用递归下降法解析并计算数学表达式。文法（优先级从低到高）：
//
//	expr    = term { ("+" | "-") term }
//	term    = unary { ("*" | "/" | "%") unary }
//	unary   = ("-" | "+") unary | power
//	power   = primary [ "^" unary ]      // 右结合，所以 2^3^2 = 2^9
//	primary = number | "(" expr ")"
func Eval(expression string) (float64, error) {
	p := &parser{src: expression}
	v, err := p.expr()
	if err != nil {
		return 0, err
	}
	if p.peek() != 0 {
		return 0, fmt.Errorf("unexpected %q at position %d", p.src[p.pos], p.pos)
	}
	if math.IsInf(v, 0) || math.IsNaN(v) {
		return 0, errors.New("result is not a finite number")
	}
	return v, nil
}

type parser struct {
	src string
	pos int
}

// peek 跳过空白并返回下一个字符，到达末尾时返回 0。
func (p *parser) peek() byte {
	for p.pos < len(p.src) && (p.src[p.pos] == ' ' || p.src[p.pos] == '\t') {
		p.pos++
	}
	if p.pos < len(p.src) {
		return p.src[p.pos]
	}
	return 0
}

func (p *parser) expr() (float64, error) {
	left, err := p.term()
	if err != nil {
		return 0, err
	}
	for {
		op := p.peek()
		if op != '+' && op != '-' {
			return left, nil
		}
		p.pos++
		right, err := p.term()
		if err != nil {
			return 0, err
		}
		if op == '+' {
			left += right
		} else {
			left -= right
		}
	}
}

func (p *parser) term() (float64, error) {
	left, err := p.unary()
	if err != nil {
		return 0, err
	}
	for {
		op := p.peek()
		if op != '*' && op != '/' && op != '%' {
			return left, nil
		}
		p.pos++
		right, err := p.unary()
		if err != nil {
			return 0, err
		}
		switch op {
		case '*':
			left *= right
		case '/', '%':
			if right == 0 {
				return 0, errors.New("division by zero")
			}
			if op == '/' {
				left /= right
			} else {
				left = math.Mod(left, right)
			}
		}
	}
}

func (p *parser) unary() (float64, error) {
	switch p.peek() {
	case '-':
		p.pos++
		v, err := p.unary()
		return -v, err
	case '+':
		p.pos++
		return p.unary()
	}
	return p.power()
}

func (p *parser) power() (float64, error) {
	base, err := p.primary()
	if err != nil {
		return 0, err
	}
	if p.peek() != '^' {
		return base, nil
	}
	p.pos++
	exp, err := p.unary()
	if err != nil {
		return 0, err
	}
	return math.Pow(base, exp), nil
}

func (p *parser) primary() (float64, error) {
	if p.peek() == '(' {
		p.pos++
		v, err := p.expr()
		if err != nil {
			return 0, err
		}
		if p.peek() != ')' {
			return 0, fmt.Errorf("missing ')' at position %d", p.pos)
		}
		p.pos++
		return v, nil
	}

	start := p.pos
	for p.pos < len(p.src) && (p.src[p.pos] >= '0' && p.src[p.pos] <= '9' || p.src[p.pos] == '.') {
		p.pos++
	}
	if start == p.pos {
		return 0, fmt.Errorf("expected number at position %d", p.pos)
	}
	v, err := strconv.ParseFloat(p.src[start:p.pos], 64)
	if err != nil {
		return 0, fmt.Errorf("invalid number %q", p.src[start:p.pos])
	}
	return v, nil
}
