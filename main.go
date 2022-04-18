package main

import (
	"fmt"
	"os"
	"path"
	"path/filepath"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/tomasz-wiszkowski/go-hookcfg/hooks"
)

type Config map[string]*Category

type Reference struct {
	category *Category
	hook     hooks.Hook
}

var kKnownHooks = Config{
	"post-commit": &Category{
		ID:   "post-commit",
		Name: "Post-commit hooks",
		Hooks: []hooks.Hook{
			hooks.CppFmt(),
			hooks.CppTidy(),
			hooks.JavaFmt(),
			hooks.GoFmt(),
			hooks.GoModTidy(),
			hooks.GoTidy(),
			hooks.PythonFmt(),
			hooks.RustFmt(),
			hooks.RustTidy(),
			hooks.ChromeClFmt(),
			hooks.ChromeClPresubmit(),
			hooks.ChromeGnDeps(),
			hooks.ChromeJsonFmt(),
			hooks.ChromeHistogramFmt(),
		},
	},
}

func add(target *tview.TreeNode, ref *Reference) {
	if ref.category == nil {
		for _, c := range kKnownHooks {
			node := tview.NewTreeNode(c.ID).SetReference(&Reference{c, nil}).SetSelectable(true).SetColor(tcell.ColorGrey)
			target.AddChild(node)
			add(node, node.GetReference().(*Reference))
		}
	} else if ref.hook == nil {
		for _, h := range ref.category.Hooks {
			node := tview.NewTreeNode("").SetReference(&Reference{ref.category, h}).SetSelectable(true)
			updateTreeNode(h, node)
			target.AddChild(node)
		}
	}
}

func updateTreeNode(hook hooks.Hook, node *tview.TreeNode) {
	var marker rune
	if hook.State() == hooks.SelectedStateUnknown {
		marker = '?'
	} else if hook.State() == hooks.SelectedStateUnavailable {
		marker = '✘'
	} else if hook.State() == hooks.SelectedStateDisabled {
		marker = ' '
	} else if hook.State() == hooks.SelectedStateEnabled {
		marker = '✔'
	}
	node.SetText(fmt.Sprintf("[%c] %s", marker, hook.Name()))
}

func onTreeNodeSelected(node *tview.TreeNode) {
	reference := node.GetReference().(*Reference)

	// Check if node or leaf. Nodes have no hook references.
	if hook := reference.hook; hook == nil {
		// This is a node.
		// Collapse if visible, expand if collapsed.
		node.SetExpanded(!node.IsExpanded())
	} else {
		// This is a leaf.
		hook.SetSelected(!hook.IsSelected())
		updateTreeNode(hook, node)
	}
}

func openRepo() *GitRepo {
	repo := GitRepoOpen()
	for _, s := range kKnownHooks {
		s.readGitSettings(repo)
	}
	return repo
}

func main() {
	self := path.Base(os.Args[0])

	if hooks, ok := kKnownHooks[self]; ok {
		runHooks(hooks)
	} else if len(os.Args) == 1 {
		showConfig()
	} else if hooks, ok := kKnownHooks[os.Args[1]]; ok {
		runHooks(hooks)
	} else if os.Args[1] == "install" {
		install()
	} else {
		fmt.Println("Unknown hook type", os.Args[1])
	}
}

func runHooks(category *Category) {
	repo := openRepo()
	files := repo.GetListOfNewAndModifiedFiles()

	// Used by hooks install, file fixing and others
	err := os.Chdir(repo.WorkDir().Root())
	if err != nil {
		panic(err)
	}

	fmt.Println("Running hooks for", category.Name)
	for _, h := range category.Hooks {
		h.Run(files)
	}
}

func showConfig() {
	repo := openRepo()
	root := tview.NewTreeNode("Hooks").SetColor(tcell.ColorGrey)
	tree := tview.NewTreeView().SetRoot(root).SetCurrentNode(root)
	tree.SetSelectedFunc(onTreeNodeSelected)
	add(root, &Reference{nil, nil})

	app := tview.NewApplication()
	app.SetRoot(tree, true).EnableMouse(true)
	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			app.Stop()
		}
		return event
	})

	if err := app.Run(); err != nil {
		panic(err)
	}

	repo.SaveConfig()
	fmt.Println("Farewell")
}

func install() {
	selfAbsolutePath,err := filepath.Abs(os.Args[0])
	if err != nil {
		panic(err)
	}
	repo := openRepo()
	gitDir := repo.GitDir()
	
	hookDir, err := gitDir.Chroot("hooks")
	if err != nil {
		panic(err)
	}
	for _, category := range kKnownHooks {
		fmt.Println("Installing", category.ID, "in", hookDir.Root(), "pointing to", selfAbsolutePath)
		err = hookDir.Remove(category.ID)
		if err != nil {
			panic(err)
		}

		err = os.Symlink(selfAbsolutePath, hookDir.Join(hookDir.Root(), category.ID))
		if err != nil {
			panic(err)
		}
	}

	showConfig()
}
