# Example usage

```yml
name: 'Pixel-perfect compare images'
uses: actions/pp
with:
  elements:
    - selector: '#root > div.myApp'
      original: 'root/div/myApp.png'
      resolution: '320x240@72' # 320px * 240px with 72dpi

    - selector: '#root > div.myApp'
      original: 'root/div/myApp.png'
      state: 'click' # hover | focus | default : none (click-right, click-left, click-middle) ?

  who-are-you: 'Jonny'
```
