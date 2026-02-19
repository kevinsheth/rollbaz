package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Project struct {
	Name  string `json:"name"`
	Token string `json:"token"`
}

type File struct {
	ActiveProject string    `json:"active_project"`
	Projects      []Project `json:"projects"`
}

type Store struct {
	path string
}

func NewStore() (*Store, error) {
	configRoot, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("resolve config dir: %w", err)
	}

	return &Store{path: filepath.Join(configRoot, "rollbaz", "config.json")}, nil
}

func NewStoreAtPath(path string) *Store {
	return &Store{path: path}
}

func (s *Store) Path() string {
	return s.path
}

func (s *Store) Load() (File, error) {
	body, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return File{}, nil
	}
	if err != nil {
		return File{}, fmt.Errorf("read config: %w", err)
	}

	var file File
	if err := json.Unmarshal(body, &file); err != nil {
		return File{}, fmt.Errorf("decode config: %w", err)
	}

	return normalize(file), nil
}

func (s *Store) Save(file File) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	normalized := normalize(file)
	body, err := json.MarshalIndent(normalized, "", "  ")
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}

	if err := os.WriteFile(s.path, append(body, '\n'), 0o600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	return nil
}

func (s *Store) AddProject(name string, token string) error {
	if strings.TrimSpace(name) == "" {
		return errors.New("project name is required")
	}
	if strings.TrimSpace(token) == "" {
		return errors.New("project token is required")
	}

	file, err := s.Load()
	if err != nil {
		return err
	}

	if index, ok := projectIndexByName(file.Projects, name); ok {
		file.Projects[index].Token = token
		if file.ActiveProject == "" {
			file.ActiveProject = name
		}
		return s.Save(file)
	}

	file.Projects = append(file.Projects, Project{Name: name, Token: token})
	if file.ActiveProject == "" {
		file.ActiveProject = name
	}

	return s.Save(file)
}

func (s *Store) RemoveProject(name string) error {
	file, err := s.Load()
	if err != nil {
		return err
	}

	filtered := make([]Project, 0, len(file.Projects))
	for _, project := range file.Projects {
		if project.Name != name {
			filtered = append(filtered, project)
		}
	}

	if len(filtered) == len(file.Projects) {
		return fmt.Errorf("project %q not found", name)
	}

	file.Projects = filtered
	if file.ActiveProject == name {
		file.ActiveProject = ""
		if len(file.Projects) > 0 {
			file.ActiveProject = file.Projects[0].Name
		}
	}

	return s.Save(file)
}

func (s *Store) UseProject(name string) error {
	file, err := s.Load()
	if err != nil {
		return err
	}

	if _, ok := projectIndexByName(file.Projects, name); ok {
		file.ActiveProject = name
		return s.Save(file)
	}

	return fmt.Errorf("project %q not found", name)
}

func (s *Store) CycleProject() (string, error) {
	file, err := s.Load()
	if err != nil {
		return "", err
	}
	if len(file.Projects) == 0 {
		return "", errors.New("no configured projects")
	}

	activeIndex := -1
	for index, project := range file.Projects {
		if project.Name == file.ActiveProject {
			activeIndex = index
			break
		}
	}

	nextIndex := 0
	if activeIndex >= 0 {
		nextIndex = (activeIndex + 1) % len(file.Projects)
	}

	file.ActiveProject = file.Projects[nextIndex].Name
	if err := s.Save(file); err != nil {
		return "", err
	}

	return file.ActiveProject, nil
}

func (s *Store) ResolveToken(projectName string) (string, string, error) {
	file, err := s.Load()
	if err != nil {
		return "", "", err
	}

	if len(file.Projects) == 0 {
		return "", "", errors.New("no configured projects")
	}

	target := projectName
	if target == "" {
		target = file.ActiveProject
	}
	if target == "" {
		return "", "", errors.New("no active project configured")
	}

	index, ok := projectIndexByName(file.Projects, target)
	if ok {
		if strings.TrimSpace(file.Projects[index].Token) == "" {
			return "", "", fmt.Errorf("project %q has no token", target)
		}
		return file.Projects[index].Token, target, nil
	}

	return "", "", fmt.Errorf("project %q not found", target)
}

func normalize(file File) File {
	trimmedProjects := make([]Project, 0, len(file.Projects))
	for _, project := range file.Projects {
		name := strings.TrimSpace(project.Name)
		if name == "" {
			continue
		}
		trimmedProjects = append(trimmedProjects, Project{Name: name, Token: strings.TrimSpace(project.Token)})
	}
	sort.Slice(trimmedProjects, func(i int, j int) bool {
		return trimmedProjects[i].Name < trimmedProjects[j].Name
	})

	return File{ActiveProject: strings.TrimSpace(file.ActiveProject), Projects: trimmedProjects}
}

func projectIndexByName(projects []Project, name string) (int, bool) {
	for index := range projects {
		if projects[index].Name == name {
			return index, true
		}
	}

	return 0, false
}
