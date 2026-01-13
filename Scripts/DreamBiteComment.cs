#if UNITY_EDITOR

using System;
using System.Collections.Generic;
using System.Linq;
using Unity.Collections;
using UnityEditor;
using UnityEngine;
using VRC.Dynamics;
using VRC.SDK3.Avatars.Components;
using VRC.SDK3.Dynamics.Constraint.Components;
using VRC.SDKBase;

namespace GhostIAm.DreamBite
{
    [AddComponentMenu("")]
    public class DreamBiteComment : MonoBehaviour, IEditorOnly
    {
        private const string _commentText = "Do not edit!\nComponent parameters will be overwritten by the DreamBite script.";

        [CustomEditor(typeof(DreamBiteComment))]
        private class CommentEditor : Editor
        {
            private static GUIContent commentContent;
            private static GUIStyle textStyle;

            public override void OnInspectorGUI()
            {
                commentContent ??= new GUIContent
                {
                    text = _commentText
                };
                textStyle ??= new GUIStyle(EditorStyles.helpBox)
                {
                    fontSize = 24,
                    fontStyle = FontStyle.Bold,
                    normal = { textColor = Color.red }
                };

                EditorGUILayout.LabelField(
                    label: GUIContent.none,
                    label2: commentContent,
                    style: textStyle,
                    options: Array.Empty<GUILayoutOption>()
                );
            }
        }
    }
}

#endif